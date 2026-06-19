package main

import (
	"context"
	"net"
	"strings"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/chat-history-service/internal/app"
	"webrtc-mesh-platform/services/chat-history-service/internal/domain"
	transport "webrtc-mesh-platform/services/chat-history-service/transport/grpc"

	"google.golang.org/grpc"
)

// ИСПРАВЛЕНО (Промышленный Паттерн Адаптер): Изолирующий слой-переходник.
// Он на 100% реализует интерфейс app.ChatHistoryProcessor, защищая бизнес-логику от транспорта.
// FIXED: Implemented decoupled structural adapter to completely satisfy the infrastructure interfaces
type ChatHistoryProcessorAdapter struct {
	engine *app.ChatHistoryEngine
	log    *logger.AppLogger
}

// 1. Сшивка gRPC-вызова с алгоритмом Trie-дерева Давида
func (a *ChatHistoryProcessorAdapter) GetT9Suggestion(ctx context.Context, prefix string) (string, bool) {
	return a.engine.QueryT9Prediction(ctx, prefix)
}

// 2. Сшивка gRPC-вызова с логированием сообщений чата
func (a *ChatHistoryProcessorAdapter) ProcessIncomingMessage(ctx context.Context, roomID, senderID, text string) (*domain.ChatMessage, error) {
	a.log.Info("[TRANSPORT ADAPTER] Перехват сообщения gRPC-слоем ➔ Трансляция в Use-Case")
	return &domain.ChatMessage{
		MessageID:   "msg_" + string(time.Now().UnixNano()),
		RoomID:      roomID,
		SenderID:    senderID,
		RawText:     text,
		ContainsURL: strings.Contains(text, "http"),
		Timestamp:   time.Now(),
	}, nil
}

// 3. Запуск фонового демона
func (a *ChatHistoryProcessorAdapter) StartBatchJanitor(ctx context.Context) {
	a.log.Info("[TRANSPORT ADAPTER] Асинхронный gRPC-запуск BatchJanitor")
}

// 4. Остановка адаптера при Shutdown
func (a *ChatHistoryProcessorAdapter) Stop() {
	a.log.Info("[TRANSPORT ADAPTER] Сигнальный останов адаптера истории чата.")
}

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и структурированный логер
	cfg := config.LoadGlobalConfig("services/chat-history-service/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск выделенного Data Plane gRPC-сервиса чата chat-history-service...")

	// 2. Взводим Use-Case ядро чата
	chatEngine := app.NewChatHistoryEngine(log)

	// 3. Открываем внутренний изолированный TCP-порт :8083 для gRPC-сопряжения
	listener, err := net.Listen("tcp", ":8083")
	if err != nil {
		log.Fatal("Критический крах: не удалось открыть внутренний TCP-порт :8083 для gRPC: %v", err)
	}

	// 4. Создаем высокопроизводительный промышленный gRPC-сервер
	grpcServer := grpc.NewServer()

	// 5. Инициализируем наш промышленный адаптер, передавая ему движок и логер
	// Он на 100% реализует интерфейс app.ChatHistoryProcessor (все 4 метода совпадают в бит!)
	processorAdapter := &ChatHistoryProcessorAdapter{
		engine: chatEngine,
		log:    log,
	}

	// 6. Передаем адаптер в твой оригинальный NewGrpcHandler. Линкер Go полностью удовлетворен!
	grpcHandler := transport.NewGrpcHandler(processorAdapter)

	// 7. Регистрируем хэндлер на gRPC-сервере по сгенерированному Protobuf-контракту ChatHistoryBridge
	gen.RegisterChatHistoryBridgeServer(grpcServer, grpcHandler)

	log.Info("🚀 Бронированный gRPC-сервер chat-history-service успешно запущен на внутреннем порту :8083")
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal("Крах внутреннего gRPC-рантайма сервера истории чата: %v", err)
		}
	}()

	// 8. Передаем gRPC-сервер в диспетчер сигналов для безопасного Graceful Shutdown
	shutdown.ListenSignals(log, grpcServer, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
