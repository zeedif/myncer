package rpc_handlers

import (
	"context"

	"github.com/hansbala/myncer/core"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

func NewUnlinkDatasourceHandler() core.GrpcHandler[
	*myncer_pb.UnlinkDatasourceRequest,
	*myncer_pb.UnlinkDatasourceResponse,
] {
	return &unlinkDatasourceImpl{}
}

type unlinkDatasourceImpl struct{}

func (u *unlinkDatasourceImpl) CheckPerms(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const,@nullable*/
	reqBody *myncer_pb.UnlinkDatasourceRequest, /*const*/
) error {
	if userInfo == nil {
		return core.NewError("user is required to unlink a datasource")
	}
	if reqBody.GetDatasource() == myncer_pb.Datasource_DATASOURCE_UNSPECIFIED {
		return core.NewError("a valid datasource must be specified")
	}
	return nil
}

func (u *unlinkDatasourceImpl) ProcessRequest(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const,@nullable*/
	reqBody *myncer_pb.UnlinkDatasourceRequest, /*const*/
) *core.GrpcHandlerResponse[*myncer_pb.UnlinkDatasourceResponse] {
	myncerCtx := core.ToMyncerCtx(ctx)

	err := myncerCtx.DB.DatasourceTokenStore.DeleteToken(ctx, userInfo.GetId(), reqBody.GetDatasource())
	if err != nil {
		return core.NewGrpcHandlerResponse_InternalServerError[*myncer_pb.UnlinkDatasourceResponse](
			core.WrappedError(err, "failed to delete datasource token"),
		)
	}

	return core.NewGrpcHandlerResponse_OK(&myncer_pb.UnlinkDatasourceResponse{})
}
