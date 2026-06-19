package app

import (
	"context"
	"runtime"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

// StartBackgroundJanitors запускает асинхронный побитовый дозор Битового Колеса Времени
func (s *SignalingService) StartBackgroundJanitors(ctx context.Context) {
	s.log.Info("Асинхронные воркеры Аппаратного Битового Колеса Времени успешно запущены...")

	// Запускаем тикер на 1 минуту, который будет нативно вращать стрелку по нашему битовому кольцу
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

// tickTimeWheelCascade осуществляет наносекундный поворот колеса времени Давида
// ИСПРАВЛЕНО (Чистая инкапсуляция пакета timewheel): Вызываем метод .Tick(), полностью
// избавив бизнес-логику от побитовой грязи и ошибок отсутствия полей TimeWheelSlots!
// FIXED: Reengineered janitor routine to poll expired keys straight from the encapsulated timewheel package
func (s *SignalingService) tickTimeWheelCascade() {
	evictedCount := 0
	now := time.Now()

	for i := uint32(0); i < s.shardCount; i++ {
		shard := s.shards[i]

		// Атомарно вращаем колесо времени конкретного шарда.
		// Метод .Tick() за 1 такт проверит бит uint64 и вернет список умерших в эту минуту ID комнат!
		expiredRoomIDs, _ := shard.wheel.Tick()
		if len(expiredRoomIDs) == 0 {
			continue
		}

		shard.mu.Lock()
		for _, roomID := range expiredRoomIDs {
			roomObj, exists := shard.lruCache.Get(roomID)
			if exists {
				room := roomObj.(*domain.VideoRoom)

				// ЖЕСТКОЕ ВЫТЕСНЕНИЕ ПО ТАЙМАУТУ: Если время сессии вышло (now >= room.UpdatedAt)
				// или комната просто опустела (0 пиров) — принудительно уничтожаем её и кикаем зал!
				if len(room.Peers) == 0 || now.After(room.UpdatedAt) || now.Equal(room.UpdatedAt) {

					// Безопасно вещаем фрейм force_kick всем участникам со звонком, временно отпустив мьютекс
					shard.mu.Unlock()
					s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "force_kick"})
					shard.mu.Lock()

					// Выжигаем дескрипторы и RAM-ноды сессии за O(1)
					shard.lruCache.Remove(roomID)
					delete(shard.conns, roomID)
					evictedCount++
				} else {
					// Страховой пояс: если люди сидят и встреча еще активна, возвращаем её на Колесо на 1 минуту вперед
					shard.wheel.Add(roomID, 1)
				}
			}
		}
		shard.mu.Unlock()
	}

	if evictedCount > 0 {
		s.log.Info("🎰 [PCEF TIME WHEEL] Время сессий исчерпано. Принудительно уничтожено %d комнат. Вызов runtime.GC().", evictedCount)
		runtime.GC()
	}
}

// forceCloseRoom терминирует сессию комнаты в LRU-кэше принудительно
func (s *SignalingService) forceCloseRoom(roomID string) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.lruCache.Remove(roomID)
	delete(shard.conns, roomID)

	// Нативно стираем ключ из побитового буфера
	shard.wheel.Remove(roomID)
}
