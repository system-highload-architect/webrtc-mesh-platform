package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

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

	maxDuration := 5 * time.Hour
	currentDuration := time.Since(room.CreatedAt)
	newTotalDuration := currentDuration + time.Duration(extendSeconds)*time.Second

	if newTotalDuration > maxDuration {
		return errors.New("cannot extend room lifecycle: total duration exceeds hard 5-hour boundary")
	}

	if newMaxPeers > 100 {
		return errors.New("cannot expand room slots: peer capacity limit cannot exceed 100 participants")
	}

	if newMaxPeers > room.MaxPeers {
		room.MaxPeers = newMaxPeers
	}

	s.log.Info("[LIMITS CONFIG] Лимиты комнаты %s успешно обновлены. Макс. участников: %d", roomID, room.MaxPeers)
	return nil
}

// IsRoomOverloadedOrPaused проверяет порог перегрузки сети в 15 человек или стейт Паузы (Req. 3)
func (s *SignalingService) IsRoomOverloadedOrPaused(roomID string) bool {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	room, exists := shard.rooms[roomID]
	if !exists {
		return false
	}

	// Порог по ТЗ: если в комнате сидит больше 15 человек, либо модератор нажал Паузу
	return len(room.Peers) > 15 || room.IsPaused
}

func (s *SignalingService) MutateSdpQuality(rawSdp string, lowBandwidth bool) string {
	lines := strings.Split(rawSdp, "\r\n")
	var mutatedLines []string

	for _, line := range lines {
		mutatedLines = append(mutatedLines, line)

		if strings.HasPrefix(line, "m=video") {
			if lowBandwidth {
				// Впрыскиваем жесткий лимит битрейта кодека AS (Application Specific)
				mutatedLines = append(mutatedLines, "b=AS:100")
			} else {
				mutatedLines = append(mutatedLines, "b=AS:2000") // 2 Mbps для HD
			}
		}
	}

	return strings.Join(mutatedLines, "\r\n")
}
