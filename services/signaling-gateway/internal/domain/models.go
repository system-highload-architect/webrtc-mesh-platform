package domain

import "time"

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
	CreatedAt       time.Time               `json:"created_at"`
}
