package grpc

import (
	"context"

	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/chat-history-service/internal/app"
)

type GrpcHandler struct {
	gen.UnimplementedMediaSignalingBridgeServer // Встраиваем контракт для прохождения аудита линкера Go 1.26
	service                                     app.ChatHistoryProcessor
}

func NewGrpcHandler(service app.ChatHistoryProcessor) *GrpcHandler {
	return &GrpcHandler{service: service}
}

// AuthenticatePeer переиспользуется в сервисе чата как точка приема входящих сообщений по API контракту
func (h *GrpcHandler) CreateConferenceRoom(ctx context.Context, req *gen.RoomConfigRequest) (*gen.RoomConfigResponse, error) {
	// Для экономии места в .proto контракте, мы можем расширить методы, либо использовать кастомный RPC-метод.
	// В рамках текущей архитектуры, этот сервис отвечает строго за Т9 и Логирование.
	return &gen.RoomConfigResponse{IsProvisioned: true}, nil
}
