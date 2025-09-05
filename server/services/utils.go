package services

import (
	"context"

	"connectrpc.com/connect"

	"github.com/hansbala/myncer/auth"
	"github.com/hansbala/myncer/core"
)

// OrchestrateHandler services as a compatibility layer between our internally implemented
// handlers and what connectrpc expects. It handles orchestration of our handler framework.
// Namely, makes sure user has perms to execute the request (by means of calling
// `CheckPerms`) and then processing of the actual request (through `ProcessRequest`).
func OrchestrateHandler[RequestT any, ResponseT any](
	ctx context.Context,
	handler core.GrpcHandler[*RequestT, *ResponseT],
	reqBody *RequestT,
) (*connect.Response[ResponseT], error) {
	userInfo := auth.UserFromContext(ctx)
	if err := handler.CheckPerms(ctx, userInfo, reqBody); err != nil {
		core.Printf("failed to check user permissions: %v", err)
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			core.WrappedError(err, "failed to check user permissions"),
		)
	}
	resp := handler.ProcessRequest(ctx, userInfo, reqBody)
	if resp.Err != nil {
		err := core.WrappedError(resp.Err, "failed to process request")
		core.Errorf(err)
		return nil, connect.NewError(
			connect.Code(resp.StatusCode), // TODO: Make sure this works as expected E2E.
			err,
		)
	}
	connectResp := connect.NewResponse(resp.Response)
	if len(resp.Cookies) > 0 {
		for _, cookie := range resp.Cookies {
			connectResp.Header().Set("Set-Cookie", cookie.String())
		}
	}
	return connectResp, nil
}

// OrchestrateStreamHandler is similar to OrchestrateHandler but for server-streaming RPCs
func OrchestrateStreamHandler[RequestT any, ResponseT any](
	ctx context.Context,
	handler core.GrpcStreamHandler[*RequestT, ResponseT],
	reqBody *RequestT,
	stream *connect.ServerStream[ResponseT],
) error {
	userInfo := auth.UserFromContext(ctx)
	if err := handler.CheckPerms(ctx, userInfo, reqBody); err != nil {
		core.Printf("failed to check user permissions: %v", err)
		return connect.NewError(
			connect.CodePermissionDenied,
			core.WrappedError(err, "failed to check user permissions"),
		)
	}

	// Create a channel to receive messages from the handler
	streamChan := make(chan *ResponseT, 10)
	
	// Run the handler in a goroutine
	go func() {
		defer close(streamChan)
		if err := handler.ProcessRequest(ctx, userInfo, reqBody, streamChan); err != nil {
			core.Errorf("failed to process stream request: %v", err)
		}
	}()

	// Send messages from the handler to the client
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-streamChan:
			if !ok {
				// Channel closed, stream is done
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}
}
