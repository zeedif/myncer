package rpc_handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/hansbala/myncer/core"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

func NewCreateSyncHandler() core.GrpcHandler[
	*myncer_pb.CreateSyncRequest,
	*myncer_pb.CreateSyncResponse,
] {
	return &createSyncImpl{}
}

type createSyncImpl struct{}

func (cs *createSyncImpl) CheckPerms(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const,@nullable*/
	reqBody *myncer_pb.CreateSyncRequest, /*const*/
) error {
	if userInfo == nil {
		return core.NewError("user is required to create a sync")
	}
	return nil
}

func (cs *createSyncImpl) ProcessRequest(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const,@nullable*/
	reqBody *myncer_pb.CreateSyncRequest, /*const*/
) *core.GrpcHandlerResponse[*myncer_pb.CreateSyncResponse] {
	if err := cs.validateRequest(ctx, reqBody, userInfo); err != nil {
		return core.NewGrpcHandlerResponse_BadRequest[*myncer_pb.CreateSyncResponse](
			core.WrappedError(err, "failed to validate create sync request"),
		)
	}

	// Create the sync from the request.
	sync, err := cs.createSyncFromRequest(reqBody, userInfo)
	if err != nil {
		return core.NewGrpcHandlerResponse_InternalServerError[*myncer_pb.CreateSyncResponse](
			core.WrappedError(err, "failed to create sync from request"),
		)
	}

	// Persist the sync to the database.
	if err := core.ToMyncerCtx(ctx).DB.SyncStore.CreateSync(ctx, sync); err != nil {
		return core.NewGrpcHandlerResponse_InternalServerError[*myncer_pb.CreateSyncResponse](
			core.WrappedError(err, "failed to create sync in database"),
		)
	}

	return core.NewGrpcHandlerResponse_OK(&myncer_pb.CreateSyncResponse{Sync: sync})
}

func (cs *createSyncImpl) validateRequest(
	ctx context.Context,
	req *myncer_pb.CreateSyncRequest, /*const*/
	userInfo *myncer_pb.User, /*const*/
) error {
	existingSyncs, err := core.ToMyncerCtx(ctx).DB.SyncStore.GetSyncs(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to check for existing syncs")
	}

	syncVariant := req.GetSyncVariant()
	switch syncVariant.(type) {
	case *myncer_pb.CreateSyncRequest_OneWaySync:
		return validateOneWaySync(ctx, userInfo, req.GetOneWaySync(), existingSyncs)
	case *myncer_pb.CreateSyncRequest_PlaylistMergeSync:
		return validatePlaylistMergeSync(ctx, userInfo, req.GetPlaylistMergeSync(), existingSyncs)
	default:
		return core.NewError("unknown sync type in validate request: %T", syncVariant)
	}
}

func (cs *createSyncImpl) createSyncFromRequest(
	req *myncer_pb.CreateSyncRequest, /*const*/
	userInfo *myncer_pb.User, /*const*/
) (*myncer_pb.Sync, error) {
	syncVariant := req.GetSyncVariant()
	switch syncVariant.(type) {
	case *myncer_pb.CreateSyncRequest_OneWaySync:
		return NewSync_OneWaySync(userInfo.GetId(), req.GetOneWaySync()), nil
	case *myncer_pb.CreateSyncRequest_PlaylistMergeSync:
		return NewSync_PlaylistMergeSync(userInfo.GetId(), req.GetPlaylistMergeSync()), nil
	default:
		return nil, core.NewError("unknown sync type in create sync from request: %T", syncVariant)
	}
}

func validateOneWaySync(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	req *myncer_pb.OneWaySync, /*const*/
	existingSyncs core.Set[*myncer_pb.Sync],
) error {
	// Validate the source and destination datasources are valid.
	if req.GetSource().GetDatasource() == myncer_pb.Datasource_DATASOURCE_UNSPECIFIED {
		return core.NewError("source datasource must be specified")
	}
	if req.GetDestination().GetDatasource() == myncer_pb.Datasource_DATASOURCE_UNSPECIFIED {
		return core.NewError("destination datasource must be specified")
	}
	// Check if the user has connected the source and destination datasources.
	connectedDatasources, err := core.ToMyncerCtx(ctx).DB.DatasourceTokenStore.GetConnectedDatasources(
		ctx,
		userInfo.GetId(),
	)
	if err != nil {
		return core.WrappedError(err, "failed to get connected datasources for user")
	}
	if !connectedDatasources.Contains(req.GetSource().GetDatasource()) {
		return core.NewError("source datasource is not connected")
	}
	if !connectedDatasources.Contains(req.GetDestination().GetDatasource()) {
		return core.NewError("destination datasource is not connected")
	}
	// Basic playlist id checks.
	if len(req.GetSource().GetPlaylistId()) == 0 {
		return core.NewError("source playlist id must be specified")
	}
	if len(req.GetDestination().GetPlaylistId()) == 0 {
		return core.NewError("destination playlist id must be specified")
	}

	for _, existingSync := range existingSyncs.ToArray() {
		if ows := existingSync.GetOneWaySync(); ows != nil {
			if ows.GetSource().GetPlaylistId() == req.GetSource().GetPlaylistId() &&
				ows.GetSource().GetDatasource() == req.GetSource().GetDatasource() &&
				ows.GetDestination().GetPlaylistId() == req.GetDestination().GetPlaylistId() &&
				ows.GetDestination().GetDatasource() == req.GetDestination().GetDatasource() {
				return core.NewError("A sync with these exact source and destination playlists already exists.")
			}
		}
	}
	return nil
}

func NewSync_OneWaySync(
	userId string, /*const*/
	oneWaySync *myncer_pb.OneWaySync, /*const*/
) *myncer_pb.Sync {
	return &myncer_pb.Sync{
		Id:     uuid.NewString(),
		UserId: userId,
		SyncVariant: &myncer_pb.Sync_OneWaySync{
			OneWaySync: oneWaySync,
		},
	}
}

func canonicalSourceKey(sources []*myncer_pb.MusicSource) string {
	keys := make([]string, len(sources))
	for i, s := range sources {
		keys[i] = fmt.Sprintf("%d:%s", s.GetDatasource(), s.GetPlaylistId())
	}
	sort.Strings(keys)
	return strings.Join(keys, "|")
}

func validatePlaylistMergeSync(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	req *myncer_pb.PlaylistMergeSync, /*const*/
	existingSyncs core.Set[*myncer_pb.Sync],
) error {
	// Validate that there are at least two sources
	if len(req.GetSources()) < 2 {
		return core.NewError("at least two source playlists are required for a merge sync")
	}
	
	// Validate the destination
	if req.GetDestination().GetDatasource() == myncer_pb.Datasource_DATASOURCE_UNSPECIFIED {
		return core.NewError("destination datasource must be specified")
	}
	if len(req.GetDestination().GetPlaylistId()) == 0 {
		return core.NewError("destination playlist id must be specified")
	}
	
	// Get user's connected datasources
	connectedDatasources, err := core.ToMyncerCtx(ctx).DB.DatasourceTokenStore.GetConnectedDatasources(
		ctx,
		userInfo.GetId(),
	)
	if err != nil {
		return core.WrappedError(err, "failed to get connected datasources for user")
	}
	
	// Validate that the destination is connected
	if !connectedDatasources.Contains(req.GetDestination().GetDatasource()) {
		return core.NewError("destination datasource is not connected")
	}
	
	// Validate each source
	for i, source := range req.GetSources() {
		if source.GetDatasource() == myncer_pb.Datasource_DATASOURCE_UNSPECIFIED {
			return core.NewError("source datasource %d must be specified", i+1)
		}
		if len(source.GetPlaylistId()) == 0 {
			return core.NewError("source playlist id %d must be specified", i+1)
		}
		if !connectedDatasources.Contains(source.GetDatasource()) {
			return core.NewError("source datasource %d is not connected", i+1)
		}
	}

	newKey := canonicalSourceKey(req.GetSources())
	newDest := req.GetDestination()

	for _, existingSync := range existingSyncs.ToArray() {
		if pms := existingSync.GetPlaylistMergeSync(); pms != nil {
			existingKey := canonicalSourceKey(pms.GetSources())
			existingDest := pms.GetDestination()
			if existingKey == newKey &&
				existingDest.GetPlaylistId() == newDest.GetPlaylistId() &&
				existingDest.GetDatasource() == newDest.GetDatasource() {
				return core.NewError("A merge sync with these exact playlists already exists.")
			}
		}
	}
	
	return nil
}

func NewSync_PlaylistMergeSync(
	userId string, /*const*/
	mergeSync *myncer_pb.PlaylistMergeSync, /*const*/
) *myncer_pb.Sync {
	return &myncer_pb.Sync{
		Id:     uuid.NewString(),
		UserId: userId,
		SyncVariant: &myncer_pb.Sync_PlaylistMergeSync{
			PlaylistMergeSync: mergeSync,
		},
	}
}
