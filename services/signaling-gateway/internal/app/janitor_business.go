package app

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"webrtc-mesh-platform/internal/pkg/backoff" // ПОДКЛЮЧАЕМ ОБЩИЙ БЭКОФФ ШАССИ
	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

// StartBackgroundJanitors запускает фоновые b2b-конвейеры мониторинга таймаутов и бэкоффа
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

func (s *SignalingService) evictIdleRoomsCascade() {
	evictedCount := 0
	now := time.Now()

	for i := uint32(0); i < s.shardCount; i++ {
		shard := s.shards[i]

		shard.mu.Lock()
		for roomID, room := range shard.rooms {
			if len(room.Peers) == 0 && now.Sub(room.CreatedAt) > 30*time.Minute {
				shard.lruCache.Remove(roomID)
				delete(shard.rooms, roomID)
				delete(shard.conns, roomID)
				evictedCount++
			}
		}
		shard.mu.Unlock()
	}

	if evictedCount > 0 {
		s.log.Info("ПАТТЕРН -> Нативно вытеснено %d пустых комнат из LRU-хвоста. Вызов runtime.GC().", evictedCount)
		runtime.GC()
	}
}

// monitorIdleRoomsBackoff шлет STIMULUS_ALERT модератору с экспоненциальной задержкой из pkg шасси
func (s *SignalingService) monitorIdleRoomsBackoff(ctx context.Context) {
	now := time.Now()

	// Инициализируем наш общий b2b-бэкофф: базовый шаг 1 минута, макс задержка 15 минут
	backoffEngine := backoff.NewExponentialBackoff(1*time.Minute, 15*time.Minute)

	for i := uint32(0); i < s.shardCount; i++ {
		shard := s.shards[i]
		shard.mu.RLock()

		for roomID, room := range shard.rooms {
			if len(room.Peers) > 0 && !room.IsPaused && now.Sub(room.CreatedAt) > 30*time.Minute {
				s.log.Info("IDLE DETECTED -> В комнате %s нет активности 30 минут. Запуск конвейера оповещений...", roomID)

				go func(rID string) {
					for attempt := 1; attempt <= 4; attempt++ {
						// ВЫЗЫВАЕМ ОБЩИЙ АЛГОРИТМ ИЗ ШАССИ С ДЖИТТЕРОМ
						backoffDuration := backoffEngine.CalculateDelay(attempt)

						select {
						case <-ctx.Done():
							return
						case <-time.After(backoffDuration):
							if s.isRoomStillIdle(rID) {
								s.log.Info("💥 BACKOFF RETRY [%d/4] -> Отправка фрейма STIMULUS_ALERT модератору комнаты %s", attempt, rID)

								// ИСПРАВЛЕНО: Применяем raw-метод вещания и структуру domain.WsSession твоего сайта
								s.broadcastToRoomRaw(rID, domain.WsSession{
									Type: "stimulus_alert",
									Text: fmt.Sprintf("Вы еще здесь? Сессия закроется автоматически через %v.", backoffDuration),
								})

								if attempt == 4 {
									s.log.Error("CRITICAL TIMEOUT -> Реакции не последовало. Принудительное схлопывание сессии %s", rID)

									// ИСПРАВЛЕНО: Применяем raw-метод вещания для финального кика по таймауту
									s.broadcastToRoomRaw(rID, domain.WsSession{Type: "force_kick"})
									s.forceCloseRoom(rID)
								}
							} else {
								return
							}
						}
					}
				}(roomID) // Убедись, что аргумент roomID передается в горутину на закрытии скобки
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

	_, exists := shard.lruCache.Get(roomID)
	return exists
}

func (s *SignalingService) forceCloseRoom(roomID string) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.lruCache.Remove(roomID)
	delete(shard.rooms, roomID)
	delete(shard.conns, roomID)
}
