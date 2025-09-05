package rpc_handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/hansbala/myncer/auth"
	"github.com/hansbala/myncer/core"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

func NewRunSyncHandler(syncEngine core.SyncEngine) core.GrpcHandler[
	*myncer_pb.RunSyncRequest,
	*myncer_pb.RunSyncResponse,
] {
	return &runSyncImpl{
		syncEngine: syncEngine,
	}
}

type runSyncImpl struct{
	syncEngine core.SyncEngine
}

func (rs *runSyncImpl) CheckPerms(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const,@nullable*/
	reqBody *myncer_pb.RunSyncRequest, /*const*/
) error {
	if userInfo == nil {
		return core.NewError("user is required to run sync job")
	}
	if err := rs.validateRequest(ctx, userInfo, reqBody); err != nil {
		return core.WrappedError(err, "failed to validate request")
	}
	return nil
}

func (rs *runSyncImpl) ProcessRequest(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const,@nullable*/
	reqBody *myncer_pb.RunSyncRequest, /*const*/
) *core.GrpcHandlerResponse[*myncer_pb.RunSyncResponse] {
	myncerCtx := core.ToMyncerCtx(ctx)
	sync, err := myncerCtx.DB.SyncStore.GetSync(ctx, reqBody.GetSyncId())
	if err != nil {
		return core.NewGrpcHandlerResponse_InternalServerError[*myncer_pb.RunSyncResponse](
			core.WrappedError(err, "could not get sync by id"),
		)
	}

	// Start the sync in a background goroutine.
	go func() {
		// Create a new background context that won't be cancelled when the HTTP request ends.
		bgCtx := context.Background()

		// Carry over the essential MyncerCtx and user info to the new context.
		bgCtx = core.WithMyncerCtx(bgCtx, myncerCtx)
		if userInfo != nil {
			bgCtx = auth.ContextWithUser(bgCtx, userInfo)
		}

		// Execute the sync. Errors are handled and logged within the engine.
		if err := rs.syncEngine.RunSync(bgCtx, userInfo, sync); err != nil {
			core.Errorf(core.WrappedError(err, "sync job for syncId %s failed", sync.GetId()))
		}
	}()

	// Immediately return a PENDING status to acknowledge the request.
	return core.NewGrpcHandlerResponse_OK(
		&myncer_pb.RunSyncResponse{
			SyncId:       sync.GetId(),
			Status:       myncer_pb.SyncStatus_SYNC_STATUS_PENDING,
			ErrorMessage: "Sync job has been queued.",
		},
	)
}

func (rs *runSyncImpl) validateRequest(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	reqBody *myncer_pb.RunSyncRequest, /*const*/
) error {
	if len(reqBody.GetSyncId()) == 0 {
		return core.NewError("sync id is required")
	}
	if _, err := uuid.Parse(reqBody.GetSyncId()); err != nil {
		return core.NewError("invalid sync id: %v", err)
	}
	sync, err := core.ToMyncerCtx(ctx).DB.SyncStore.GetSync(ctx, reqBody.GetSyncId())
	if err != nil {
		return core.WrappedError(err, "could not get sync with id: %s", reqBody.GetSyncId())
	}
	if userInfo.GetId() != sync.GetUserId() {
		return core.NewError(
			"user %s does not have permission to run sync %s",
			userInfo.GetId(),
			reqBody.GetSyncId(),
		)
	}
	return nil
}

