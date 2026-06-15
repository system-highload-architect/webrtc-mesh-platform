package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

// CreateRoom инициализирует комнату за O(1) и регистрирует её в наносекундном LRU-кэше (Req. 5)
func (s *SignalingService) CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	room := &domain.VideoRoom{
		RoomID:    roomID,
		MaxPeers:  maxPeers,
		IsPaused:  false,
		Peers:     make(map[string]*domain.PeerSession),
		CreatedAt: time.Now(),
	}

	shard.rooms[roomID] = room
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}

	// Фиксируем комнату в структуре LRU-кэша. Если лимит в 1000 превышен — старый хвост атомарно вытеснится
	shard.lruCache.Set(roomID, room)

	mac := hmac.New(sha256.New, s.hmacSecret)
	mac.Write([]byte(roomID + fmt.Sprintf("%d", time.Now().UnixNano())))
	token := hex.EncodeToString(mac.Sum(nil))

	return token, nil
}
