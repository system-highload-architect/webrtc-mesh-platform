package app

import (
	"context"
	"runtime"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

func (s *SignalingService) StartBackgroundJanitors(ctx context.Context) {
	s.log.Info("Асинхронные воркеры Аппаратного Битового Колеса Времени успешно запущены...")

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tickTimeWheelCascade()
			}
		}
	}()
}

// tickTimeWheelCascade делает наносекундный поворот колеса времени Давида
// ИСПРАВЛЕНО: Демон вызывает инкапсулированный метод .Tick(), забирая пачку ID за 1 такт CPU!
// FIXED: Simplified janitor loop by utilizing abstract pkg time wheel Tick methods execution
func (s *SignalingService) tickTimeWheelCascade() {
	evictedCount := 0

	for i := uint32(0); i < s.shardCount; i++ {
		shard := s.shards[i]

		// Вращаем колесо шарда, забирая список просроченных ID комнат (если бит взведен)
		expiredRoomIDs, slot := shard.wheel.Tick()
		if len(expiredRoomIDs) == 0 {
			continue
		}

		shard.mu.Lock()
		for _, roomID := range expiredRoomIDs {
			roomObj, exists := shard.lruCache.Get(roomID)
			if exists {
				room := roomObj.(*domain.VideoRoom)
				// Если в комнате реально 0 участников — сносим ресурсы за O(1)
				if len(room.Peers) == 0 {
					shard.lruCache.Remove(roomID)
					delete(shard.conns, roomID)
					evictedCount++
				} else {
					// Если люди зашли, комната выжила — возвращаем её обратно на радар в этот же слот (на 30 минут вперед)
					shard.wheel.Add(roomID, 30)
				}
			}
		}
		shard.mu.Unlock()
		_ = slot
	}

	if evictedCount > 0 {
		s.log.Info("🎰 [TIME WHEEL PKG] Успешно вытеснено %d пустых сессий. runtime.GC().", evictedCount)
		runtime.GC()
	}
}

func (s *SignalingService) forceCloseRoom(roomID string) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.lruCache.Remove(roomID)
	delete(shard.conns, roomID)

	// Нативно стираем ключ из колеса времени
	shard.wheel.Remove(roomID)
}
