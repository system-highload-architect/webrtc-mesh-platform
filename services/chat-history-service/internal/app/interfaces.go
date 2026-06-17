package app

import (
	"context"
	"webrtc-mesh-platform/services/chat-history-service/internal/domain"
)

// ChatHistoryProcessor задает контракт асинхронного конвейера обработки сообщений чата
type ChatHistoryProcessor interface {
	ProcessIncomingMessage(ctx context.Context, roomID, senderID, text string) (*domain.ChatMessage, error)
	GetRoomChatHistory(ctx context.Context, roomID string) ([]*domain.ChatMessage, error) // ДОБАВЛЕНО (gRPC Выгрузка Архива логов)
	GetT9Suggestion(ctx context.Context, prefix string) (string, bool)
	StartBatchJanitor(ctx context.Context)
	Stop()
}
