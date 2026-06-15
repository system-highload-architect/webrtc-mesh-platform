package domain

import "time"

// ChatMessage описывает структуру текстового лога переписки в рамках DDD-контура
type ChatMessage struct {
	MessageID   string    `json:"message_id"`
	RoomID      string    `json:"room_id"`
	SenderID    string    `json:"sender_id"`
	RawText     string    `json:"raw_text"`     // Текст после AppSec санитизации
	ContainsURL bool      `json:"contains_url"` // Флаг наличия внешних ссылок
	Timestamp   time.Time `json:"timestamp"`
}
