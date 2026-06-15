package app

import (
	"context"
)

// RoomManagerEngine задает b2b-контракт для управления стейтом WebRTC Mesh комнат
type RoomManagerEngine interface {
	CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error)
	GetAutocompleteSuggestion(ctx context.Context, prefix string) (string, bool)
	BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error
}
