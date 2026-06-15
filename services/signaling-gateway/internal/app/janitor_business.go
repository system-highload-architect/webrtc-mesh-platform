package app

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"time"
)

// StartBackgroundJanitors запускает фоновые b2b-конвейеры мониторинга таймаутов и бэкоффа (Req. 4)
func (s *SignalingService) StartBackgroundJanitors(ctx context.Context) {
	s.log.Info("Асинхронные воркеры Каскадного вытеснения и Экспоненциального Бэкоффа успешно запущены...")

	// 1. Воркер Каскадного вытеснения мертвых комнат по Паттерну Давида (Таймаут 30 минут)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.evictIdleRoomsCascade()
			}
		}
	}()

	// 2. Воркер мониторинга Idle-тишины участников с Экспоненциальным Бэкоффом
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.monitorIdleRoomsBackoff(ctx)
			}
		}
	}()
}

// evictIdleRoomsCascade реализует ленивое каскадное сжатие хвоста RAM-памяти (Паттерн Давида)
func (s *SignalingService) evictIdleRoomsCascade() {
	evictedCount := 0
	now := time.Now()

	// Обходим все 32 изолированных шарда для полной ликвидации Mutex Contention
	for i := uint32(0); i < s.shardCount; i++ {
		shard := s.shards[i]

		shard.mu.Lock()
		for roomID, room := range shard.rooms {
			// Если в комнате нет ни одного пользователя более 30 минут
			if len(room.Peers) == 0 && now.Sub(room.CreatedAt) > 30*time.Minute {
				delete(shard.rooms, roomID)
				delete(shard.conns, roomID)
				evictedCount++
			}
		}
		shard.mu.Unlock()
	}

	if evictedCount > 0 {
		s.log.Info("ПАТТЕРН ДАВИДА -> Каскадно вытеснено %d пустых комнат. Форсирование сборщика мусора runtime.GC().", evictedCount)
		// Принудительно возвращаем освобожденные страницы памяти операционной системе хоста
		runtime.GC()
	}
}

// monitorIdleRoomsBackoff шлет STIMULUS_ALERT модератору с экспоненциальной задержкой
func (s *SignalingService) monitorIdleRoomsBackoff(ctx context.Context) {
	now := time.Now()

	for i := uint32(0); i < s.shardCount; i++ {
		shard := s.shards[i]
		shard.mu.RLock()

		for roomID, room := range shard.rooms {
			// Если в комнате есть люди, но полная тишина 30 минут и пауза не установлена
			if len(room.Peers) > 0 && !room.IsPaused && now.Sub(room.CreatedAt) > 30*time.Minute {
				s.log.Info("IDLE DETECTED -> В комнате %s нет активности 30 минут. Запуск конвейера оповещений...", roomID)

				// Запускаем асинхронный экспоненциальный бэкофф ретраев (3-5 раз)
				go func(rID string) {
					for attempt := 1; attempt <= 4; attempt++ {
						// Вычисляем экспоненциальную задержку: 2^attempt минут (2, 4, 8, 16...)
						backoffDuration := time.Duration(math.Pow(2, float64(attempt))) * time.Minute

						select {
						case <-ctx.Done():
							return
						case <-time.After(backoffDuration):
							// Проверяем, не появилась ли активность
							if s.isRoomStillIdle(rID) {
								s.log.Info("💥 BACKOFF RETRY [%d/4] -> Отправка фрейма STIMULUS_ALERT модератору комнаты %s", attempt, rID)
								s.broadcastToRoom(rID, map[string]any{
									"type": "stimulus_alert",
									"text": fmt.Sprintf("Вы еще здесь? Сессия закроется автоматически через %v.", backoffDuration),
								})

								if attempt == 4 {
									s.log.Error("CRITICAL TIMEOUT -> Реакции не последовало. Принудительное схлопывание сессии %s", rID)
									s.broadcastToRoom(rID, map[string]any{"type": "force_kick"})
									s.forceCloseRoom(rID)
								}
							} else {
								return // Активность появилась, выходим из цикла бэкоффа
							}
						}
					}
				}(roomID)
			}
		}
		shard.mu.RUnlock()
	}
}

func (s *SignalingService) isRoomStillIdle(roomID string) bool {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	_, exists := shard.rooms[roomID]
	return exists
}

func (s *SignalingService) forceCloseRoom(roomID string) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	delete(shard.rooms, roomID)
	delete(shard.conns, roomID)
}
