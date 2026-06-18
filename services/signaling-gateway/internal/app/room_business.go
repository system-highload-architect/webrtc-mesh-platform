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

// CreateRoom инициализирует комнату в LRU-кэше и генерирует защищенный Anti-Replay HMAC-токен (Req. 5)
func (s *SignalingService) CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	room := &domain.VideoRoom{
		RoomID:    roomID,
		MaxPeers:  int(maxPeers),
		IsPaused:  false,
		Peers:     make(map[string]*domain.PeerSession),
		CreatedAt: time.Now(),
	}

	shard.rooms[roomID] = room
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}

	shard.lruCache.Set(roomID, room)

	// ПАТТЕРН БЕЗОПАСНОСТИ (Анти-Реплей): Вшиваем текущий Unix-таймстамп (TTL) в тело подписи
	// Это предотвратит атаки повторного воспроизведения старых инвайт-ссылок хакерами
	timestampPayload := fmt.Sprintf("%d", time.Now().Unix())

	mac := hmac.New(sha256.New, s.hmacSecret)
	mac.Write([]byte(roomID + ":" + timestampPayload))
	tokenSignature := hex.EncodeToString(mac.Sum(nil))

	// Финальный токен содержит payload времени и саму подпись, разделенные двоеточием
	secureInviteToken := fmt.Sprintf("%s:%s", timestampPayload, tokenSignature)

	s.log.Info("[SECURITY SHIELD] Успешно сгенерирован Anti-Replay токен для комнаты %s", roomID)
	return secureInviteToken, nil
}

// FetchIceServersConfig возвращает конфигурацию Coturn TURN кластера для обхода Symmetric NAT
func (s *SignalingService) FetchIceServersConfig() map[string]any {
	return map[string]any{
		"iceServers": []map[string]any{
			{
				"urls": []string{"stun:stun.l.google.com:19302", "stun:stun.l.google.com:19302"},
			},
			{
				// Промышленный Coturn TURN-сервер для пробивки корпоративных Firewall и симметричных NAT
				"urls":       []string{"turn:coturn.clearway.ru:3478?transport=udp"},
				"username":   "webrtc_b2b_user",
				"credential": "pki_secure_password_2026",
			},
		},
	}
}
