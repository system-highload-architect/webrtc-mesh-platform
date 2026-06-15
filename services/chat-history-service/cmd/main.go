package main

import (
	"context"
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/chat-history-service/internal/app"
	transport "webrtc-mesh-platform/services/chat-history-service/transport/grpc"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем локальный контур конфигурации и structured логер
	cfg := config.LoadConfig("services/chat-history-service/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск асинхронного аналитического сервиса истории чата и Т9-движка...")

	// 2. Взводим Use-Case слой (Пакетный дисковый сборщик + Trie Tree)
	var historyCore app.ChatHistoryProcessor = app.NewHistoryService(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Нативно запускаем бесконечный фоновый воркер накопления пачек по 30 штук (Req. 4)
	historyCore.StartBatchJanitor(ctx)

	// 3. Собираем gRPC-транспорт чат-моста
	grpcHandler := transport.NewGrpcHandler(historyCore)

	server := grpc.NewServer()
	gen.RegisterChatHistoryBridgeServer(server, grpcHandler)

	// 4. Открываем сетевой сокет на прослушивание порта :50057
	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}

	go func() {
		log.Info("gRPC Chat-History сервер успешно запущен на %s", cfg.BindAddr)
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера истории чата: %v", err)
		}
	}()

	// 5. Плавная остановка (Graceful Shutdown) с принудительным коммитом остатков логов из RAM на диск
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
