package grpc

import (
	"context"

	gen "webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/signaling-gateway/internal/app"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	gen.UnimplementedMediaSignalingBridgeServer
	service app.RoomManagerEngine
}

func NewGrpcHandler(service app.RoomManagerEngine) *GrpcHandler {
	return &GrpcHandler{service: service}
}

func (h *GrpcHandler) CreateConferenceRoom(ctx context.Context, req *gen.RoomConfigRequest) (*gen.RoomConfigResponse, error) {
	if req.RoomId == "" {
		return nil, status.Error(codes.InvalidArgument, "Room ID descriptor cannot be empty")
	}

	token, err := h.service.CreateRoom(ctx, req.RoomId, req.MaxSubscribers)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to provision room state: %v", err)
	}

	return &gen.RoomConfigResponse{
		IsProvisioned:   true,
		HmacAccessToken: token,
		StatusCode:      2001, // DIAMETER_SUCCESS style
	}, nil
}

func (h *GrpcHandler) BroadcastControlFrame(ctx context.Context, req *gen.ControlFramePayload) (*gen.ControlFrameAck, error) {
	err := h.service.BroadcastControlMessage(ctx, req.RoomId, req.CommandType, req.TargetPeerId)
	if err != nil {
		return &gen.ControlFrameAck{IsDispatched: false}, nil
	}
	return &gen.ControlFrameAck{IsDispatched: true}, nil
}
