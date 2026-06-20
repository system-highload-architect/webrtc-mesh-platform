package app

import (
	"context"

	"webrtc-mesh-platform/services/chat-history-service/internal/domain"
)

// ChatHistoryProcessor задает контракт асинхронного конвейера обработки сообщений чата
type ChatHistoryProcessor interface {
	ProcessIncomingMessage(ctx context.Context, roomID, senderID, text string) (*domain.ChatMessage, error)
	GetT9Suggestion(ctx context.Context, prefix string) (string, bool)
	StartBatchJanitor(ctx context.Context)
	Stop()
}
