package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

// UpdateRoomLimits позволяет модератору продлить конференцию или расширить слоты участников (Req. 2)
func (s *SignalingService) UpdateRoomLimits(ctx context.Context, roomID string, extendSeconds int64, newMaxPeers int32) error {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	roomObj, exists := shard.lruCache.Get(roomID)
	if !exists {
		return fmt.Errorf("room %s not found in lru index", roomID)
	}
	room := roomObj.(*domain.VideoRoom)

	// Жесткий b2b-лимит по ТЗ: суммарное время конференции не может превышать 5 часов
	maxDuration := 5 * time.Hour
	currentDuration := time.Since(room.CreatedAt)
	newTotalDuration := currentDuration + time.Duration(extendSeconds)*time.Second

	if newTotalDuration > maxDuration {
		return errors.New("cannot extend room lifecycle: total duration exceeds hard 5-hour boundary")
	}

	// Ограничение по ТЗ: не более 100 человек в одной Mesh-комнате
	if newMaxPeers > 100 {
		return errors.New("cannot expand room slots: peer capacity limit cannot exceed 100 participants")
	}

	if newMaxPeers > room.MaxPeers {
		room.MaxPeers = newMaxPeers
	}

	s.log.Info("[LIMITS CONFIG] Лимиты комнаты %s успешно обновлены. Макс. участников: %d", roomID, room.MaxPeers)
	return nil
}

// MutateSdpQuality манипулирует строками кодеков SDP контракта для авто-скалирования трафика (Req. 3)
func (s *SignalingService) MutateSdpQuality(rawSdp string, lowBandwidth bool) string {
	lines := strings.Split(rawSdp, "\r\n")
	var mutatedLines []string

	for _, line := range lines {
		mutatedLines = append(mutatedLines, line)

		// Находим секцию описания видео-медиа (m=video)
		if strings.HasPrefix(line, "m=video") {
			if lowBandwidth {
				// ПРИНУДИТЕЛЬНАЯ SDP МУТАЦИЯ (Req. 3): Выставляем жесткий потолок битрейта в 100 Кбит/с
				// Это активирует режим Muted Keyframes на стороне WebRTC-стека браузеров
				mutatedLines = append(mutatedLines, "b=AS:100") // Application Specific Bandwidth limit
				s.log.Info("[SDP MUTATION] Контракт переведен в режим Low-Bandwidth Muted Keyframes (100 Kbps limit)")
			} else {
				// Стандартный b2b HD профиль (2000 Кбит/с)
				mutatedLines = append(mutatedLines, "b=AS:2000")
			}
		}
	}

	return strings.Join(mutatedLines, "\r\n")
}
