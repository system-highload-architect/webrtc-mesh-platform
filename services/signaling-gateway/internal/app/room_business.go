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

// CreateRoom инициализирует комнату за O(1) и генерирует HMAC-SHA256 токен для защиты от CSRF (Req. 5)
func (s *SignalingService) CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	// Жестко локально лочим только один шард для обхода Mutex Contention
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Инициализируем стейт комнаты в RAM
	shard.rooms[roomID] = &domain.VideoRoom{
		RoomID:    roomID,
		MaxPeers:  maxPeers,
		IsPaused:  false,
		Peers:     make(map[string]*domain.PeerSession),
		CreatedAt: time.Now(),
	}

	// Если пула коннекшенов для этой комнаты еще нет — аллоцируем память
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}

	// ПАТТЕРН БЕЗОПАСНОСТИ (Req. 5): Генерация криптографического HMAC-SHA256 инвайт-токена
	mac := hmac.New(sha256.New, s.hmacSecret)
	mac.Write([]byte(roomID + fmt.Sprintf("%d", time.Now().UnixNano())))
	token := hex.EncodeToString(mac.Sum(nil))

	return token, nil
}
