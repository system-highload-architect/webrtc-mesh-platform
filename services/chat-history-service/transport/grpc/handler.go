package grpc

import (
	"context"

	"webrtc-mesh-platform/pb/gen" // Сгенерированные gRPC контракты чата
	"webrtc-mesh-platform/services/chat-history-service/internal/app"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	// Анонимно встраиваем нереализованный сервер чата для прохождения strict-аудита линкера Go 1.26
	gen.UnimplementedChatHistoryBridgeServer
	service app.ChatHistoryProcessor
}

func NewGrpcHandler(service app.ChatHistoryProcessor) *GrpcHandler {
	return &GrpcHandler{service: service}
}

// IngestChatMessage принимает фрейм текста, прогоняет через XSS-фильтры и пушит в Batch-канал (Req. 4)
func (h *GrpcHandler) IngestChatMessage(ctx context.Context, req *gen.ChatMessagePayload) (*gen.ChatMessageAck, error) {
	if req.MessageText == "" {
		return nil, status.Error(codes.InvalidArgument, "Chat message text body cannot be empty")
	}

	// Вызываем Use-Case слой бизнес-логики через интерфейс
	msg, err := h.service.ProcessIncomingMessage(ctx, req.RoomId, req.SenderId, req.MessageText)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ingest message context: %v", err)
	}

	return &gen.ChatMessageAck{
		MessageId:     msg.MessageID,
		SanitizedText: msg.RawText,
		ContainsUrl:   msg.ContainsURL,
	}, nil
}

// QueryT9Autocomplete осуществляет наносекундный поиск Т9-подсказки в Trie-дереве за O(K)
func (h *GrpcHandler) QueryT9Autocomplete(ctx context.Context, req *gen.T9QueryRequest) (*gen.T9QueryResponse, error) {
	suggestion, found := h.service.GetT9Suggestion(ctx, req.Prefix)

	return &gen.T9QueryResponse{
		Suggestion: suggestion,
		IsFound:    found,
	}, nil
}
