package grpc

import (
	"context"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/chat-history-service/internal/app"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	gen.UnimplementedChatHistoryBridgeServer // Наш скомпилированный 5-сервисный мост чата
	service                                  app.ChatHistoryProcessor
}

func NewGrpcHandler(service app.ChatHistoryProcessor) *GrpcHandler {
	return &GrpcHandler{service: service}
}

func (h *GrpcHandler) IngestChatMessage(ctx context.Context, req *gen.ChatMessagePayload) (*gen.ChatMessageAck, error) {
	if req.MessageText == "" {
		return nil, status.Error(codes.InvalidArgument, "Chat message text body cannot be empty")
	}
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

func (h *GrpcHandler) QueryT9Autocomplete(ctx context.Context, req *gen.T9QueryRequest) (*gen.T9QueryResponse, error) {
	suggestion, found := h.service.GetT9Suggestion(ctx, req.Prefix)
	return &gen.T9QueryResponse{
		Suggestion: suggestion,
		IsFound:    found,
	}, nil
}

// ИСПРАВЛЕНО (DDD gRPC Мост): Реализуем вычитку бинарного лога с NVMe диска по новому gRPC контракту
// FIXED: Bound proto-defined GetRoomChatHistory transaction to pipe log arrays over to signaling stream
func (h *GrpcHandler) GetRoomChatHistory(ctx context.Context, req *gen.GetHistoryRequest) (*gen.GetHistoryResponse, error) {
	messages, err := h.service.GetRoomChatHistory(ctx, req.RoomId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to scan history binary segments from disk: %v", err)
	}

	var pbLogs []*gen.ChatLogFrame
	for _, m := range messages {
		pbLogs = append(pbLogs, &gen.ChatLogFrame{
			SenderId: m.SenderID,
			Text:     m.RawText,
		})
	}
	return &gen.GetHistoryResponse{Logs: pbLogs}, nil
}
