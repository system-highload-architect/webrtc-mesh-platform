package domain

import (
	"encoding/json"
	"time"
)

type PeerSession struct {
	PeerID        string    `json:"peer_id"`
	IsModerator   bool      `json:"is_moderator"`
	IsMuted       bool      `json:"is_muted"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

type VideoRoom struct {
	RoomID          string                  `json:"room_id"`
	MaxPeers        int32                   `json:"max_peers"`
	IsPaused        bool                    `json:"is_paused"`
	ActiveSpeakerID string                  `json:"active_speaker_id"`
	Peers           map[string]*PeerSession `json:"peers"`
	ChatHistory     []map[string]any        `json:"chat_history"`
	CreatedAt       time.Time               `json:"created_at"`
}

// WsSession описывает единую b2b структуру обмена фреймами в Full-Mesh сети
type WsSession struct {
	Type         string          `json:"type"`
	RoomID       string          `json:"room_id,omitempty"`
	SenderID     string          `json:"sender_id,omitempty"`
	SenderName   string          `json:"sender_name,omitempty"`
	TargetID     string          `json:"target_id,omitempty"`
	Text         string          `json:"text,omitempty"`
	Command      string          `json:"command,omitempty"`        // Плоское поле для Паузы, Mute и Kick
	TargetPeerID string          `json:"target_peer_id,omitempty"` // Плоское поле для ID цели модерации
	Payload      json.RawMessage `json:"payload,omitempty"`        // Нативный массив байт для SDP Offer/Answer и ICE
}
