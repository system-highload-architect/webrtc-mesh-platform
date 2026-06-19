package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex" // ИСПРАВЛЕНО: Восстановили корректный стандартный путь импорта
	"fmt"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

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

	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}

	shard.lruCache.Set(roomID, room)

	// Нативно добавляем комнату на 30 минут в инкапсулированное колесо за O(1)
	slot := shard.wheel.Add(roomID, 30)

	timestampPayload := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, s.hmacSecret)
	mac.Write([]byte(roomID + ":" + timestampPayload))
	tokenSignature := hex.EncodeToString(mac.Sum(nil))

	s.log.Info("[TIME WHEEL] Комната %s поставлена на радар в слот %d", roomID, slot)
	return fmt.Sprintf("%s:%s", timestampPayload, tokenSignature), nil
}

func (s *SignalingService) ExtendRoomDuration(roomID string, extendMinutes int) error {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	roomObj, exists := shard.lruCache.Get(roomID)
	if !exists {
		return fmt.Errorf("conference room expired or missing")
	}
	room := roomObj.(*domain.VideoRoom)

	totalLifetimeMinutes := int(time.Since(room.CreatedAt).Minutes()) + extendMinutes
	if totalLifetimeMinutes >= 300 {
		return fmt.Errorf("🔒 [AppSec Guard]: Достигнут лимит 5 часов. Продление отклонено")
	}

	// Атомарно переносим комнату по битовому кольцу через метод .Move() за O(1)
	newSlot, _ := shard.wheel.Move(roomID, extendMinutes)

	s.log.Info("[TIME WHEEL] Сессия %s успешно продлена по кольцу в слот %d", roomID, newSlot)
	return nil
}

// FetchIceServersConfig возвращает конфигурацию Coturn TURN кластера для обхода Symmetric NAT
// ИСПРАВЛЕНО (Синтаксис вложенных мап Go): Полностью исправили синтаксическую ошибку missing key in map literal!
// FIXED: Corrected multi-layered inline map literal declarations to pass strict go compiler audits
func (s *SignalingService) FetchIceServersConfig() map[string]any {
	return map[string]any{
		"iceServers": []map[string]any{
			{
				"urls": []string{"stun:stun.l.google.com:19302"},
			},
			{
				// Промышленный Coturn TURN-сервер для пробивки корпоративных Firewall компании
				"urls":       []string{"turn:coturn.clearway.ru:3478?transport=udp"},
				"username":   "webrtc_b2b_user",
				"credential": "pki_secure_password_2026",
			},
		},
	}
}
