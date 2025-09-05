
package rpc_handlers

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hansbala/myncer/auth"
	"github.com/hansbala/myncer/core"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

// Handler directo para streaming de sync status
func SubscribeToSyncStatus(
	ctx context.Context,
	req *connect.Request[myncer_pb.SubscribeToSyncStatusRequest],
	stream *connect.ServerStream[myncer_pb.SyncRun],
) error {
	userInfo := auth.UserFromContext(ctx)
	if userInfo == nil {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("user is required to subscribe to sync status"))
	}

	if len(req.Msg.GetSyncId()) == 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("sync id is required"))
	}
	if _, err := uuid.Parse(req.Msg.GetSyncId()); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid sync id"))
	}

	myncerCtx := core.ToMyncerCtx(ctx)
	sync, err := myncerCtx.DB.SyncStore.GetSync(ctx, req.Msg.GetSyncId())
	if err != nil {
		return connect.NewError(connect.CodeNotFound, core.WrappedError(err, "could not find sync with id: %s", req.Msg.GetSyncId()))
	}
	if userInfo.GetId() != sync.GetUserId() {
		return connect.NewError(connect.CodePermissionDenied, errors.New("user does not have permission to subscribe to this sync"))
	}

	syncId := req.Msg.GetSyncId()

	// Obtener el estado más reciente para enviarlo inmediatamente y para el heartbeat
	var mostRecentRun *myncer_pb.SyncRun
	syncRuns, err := myncerCtx.DB.SyncRunStore.GetSyncs(ctx, nil, core.NewSet(syncId))
	if err != nil {
		return connect.NewError(connect.CodeInternal, core.WrappedError(err, "failed to get initial sync runs"))
	}
	for _, run := range syncRuns.ToArray() {
		if mostRecentRun == nil || run.GetUpdatedAt().AsTime().After(mostRecentRun.GetUpdatedAt().AsTime()) {
			mostRecentRun = run
		}
	}
	if mostRecentRun != nil {
		if err := stream.Send(mostRecentRun); err != nil {
			return err
		}
	}

	subscription := myncerCtx.SyncStatusBroadcaster.Subscribe(syncId)
	defer myncerCtx.SyncStatusBroadcaster.Unsubscribe(syncId, subscription)
	core.Printf("Client subscribed to sync status for sync ID: %s", syncId)

	ticker := time.NewTicker(30 * time.Second) // Envía un ping cada 30 segundos
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			core.Printf("Client disconnected from sync status stream for sync ID: %s", syncId)
			return ctx.Err()
		case syncRun, ok := <-subscription:
			if !ok {
				return nil
			}
			mostRecentRun = syncRun // Actualizar el estado más reciente
			if err := stream.Send(syncRun); err != nil {
				core.Errorf(core.WrappedError(err, "failed to send sync run update to client for sync ID: %s", syncId))
				return err
			}
		case <-ticker.C:
			// Si hay un estado que enviar y la conexión sigue viva, lo enviamos como heartbeat.
			if mostRecentRun != nil {
				if err := stream.Send(mostRecentRun); err != nil {
					core.Errorf(core.WrappedError(err, "failed to send heartbeat for sync ID: %s", syncId))
					return err // La conexión probablemente se cerró, así que salimos.
				}
			}
		}
	}
}
