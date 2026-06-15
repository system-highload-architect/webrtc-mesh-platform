package app

import (
	"context"

	"github.com/gorilla/websocket"
)

// RoomManagerEngine описывает строгий b2b-контракт для тестов и абстракции
type RoomManagerEngine interface {
	CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error)
	HandleWsSignal(roomID, tokenStr string, ws *websocket.Conn)
	BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error
}
