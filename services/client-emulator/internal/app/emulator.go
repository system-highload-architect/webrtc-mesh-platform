package app

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/pb/gen"
)

type LoadEmulator struct {
	log          *logger.AppLogger
	signalingCli gen.MediaSignalingBridgeClient
	chatCli      gen.ChatHistoryBridgeClient
	concurrency  int
	interval     time.Duration
	brokenWords  []string
}

func NewLoadEmulator(log *logger.AppLogger, sigCli gen.MediaSignalingBridgeClient, chatCli gen.ChatHistoryBridgeClient, concurrency int, interval time.Duration) *LoadEmulator {
	return &LoadEmulator{
		log:          log,
		signalingCli: sigCli,
		chatCli:      chatCli,
		concurrency:  concurrency,
		interval:     interval,
		// Массив фреймов с ошибочной раскладкой для проверки Trie-T9 транслита (Req. 4)
		brokenWords: []string{"ghbdtn", "gvhjnjrjk", "fhudntrnshf", "rjyathtywbz", "kjubhjdfybt"},
	}
}

// StartSpawningBlast запускает параллельные горутины, штурмующие gRPC сокеты кластера
func (e *LoadEmulator) StartSpawningBlast(ctx context.Context) {
	e.log.Info("Запуск стресс-тестирования WebRTC-Mesh кластера. Потоков: %d", e.concurrency)

	for i := 0; i < e.concurrency; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			peerID := fmt.Sprintf("peer_bot_%d_%d", i, time.Now().UnixNano()%1000)
			roomID := fmt.Sprintf("room_stress_test_%d", i%5) // Переиспользуем 5 комнат

			// Разворачиваем изолированный виртуальный смартфон/браузер в горутине
			go e.simulateSinglePeerLoop(ctx, roomID, peerID)
			time.Sleep(e.interval)
		}
	}
}

func (e *LoadEmulator) simulateSinglePeerLoop(ctx context.Context, roomID string, peerID string) {
	// 1. Имитируем сигнальный Handshake: отправляем запрос на создание/вход в комнату
	resp, err := e.signalingCli.CreateConferenceRoom(ctx, &gen.RoomConfigRequest{
		RoomId:          roomID,
		MaxSubscribers:  100,
		DurationSeconds: 3600,
	})
	if err != nil {
		e.log.Error("[%s] Ошибка Gx-сигнализации создания комнаты: %v", peerID, err)
		return
	}

	e.log.Info("[%s] Успешный Handshake. Получен HMAC CSRF-токен: %s...", peerID, resp.HmacAccessToken[:10])

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// СЛУЧАЙНЫЙ ВЫБОР ФУНКЦИИ ТЕСТИРОВАНИЯ
			action := rand.Intn(2)

			if action == 0 {
				// Тест 1: Имитируем ввод текста сбитой раскладки. Проверяем наносекундное Trie-T9 дерево
				word := e.brokenWords[rand.Intn(len(e.brokenWords))]
				t9Resp, err := e.chatCli.QueryT9Autocomplete(ctx, &gen.T9QueryRequest{Prefix: word})
				if err == nil && t9Resp.IsFound {
					e.log.Info("[%s] Trie-T9 MATCH! Ввод: '%s' ➔ Автодополнение: '%s'", peerID, word, t9Resp.Suggestion)
				}
			} else {
				// Тест 2: Пуш сообщения чата. Проверяем XSS-экранирование, Safe Link Proxy и Batch INSERT
				text := "Легитимный b2b фрейм контента. Ссылка: https://github.com"
				if rand.Intn(5) == 0 {
					text = "<script>alert('XSS_ATTACK')</script> Флуд-пакет" // Имитируем хакерский обход фронта
				}

				ack, err := e.chatCli.IngestChatMessage(ctx, &gen.ChatMessagePayload{
					RoomId:      roomID,
					SenderId:    peerID,
					MessageText: text,
				})
				if err == nil {
					e.log.Info("[%s] Chat Ingest Ack! MessageID: %s | Санитизированный текст: %s", peerID, ack.MessageId[:12], ack.SanitizedText[:30])
				}
			}
		}
	}
}
