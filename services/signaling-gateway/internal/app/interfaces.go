package app

import (
	"context"

	"github.com/gorilla/websocket"
)

type RoomManagerEngine interface {
	CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error)
	HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool)
	UpdateRoomLimits(ctx context.Context, roomID string, extendSeconds int64, newMaxPeers int32) error
	MutateSdpQuality(rawSdp string, lowBandwidth bool) string
	BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error
	StartServerRecording(roomID string) (string, string)
	WriteMediaFrame(roomID string, rawPayload string)
	StopServerRecording(roomID string)
}
