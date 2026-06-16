package main

import (
	"context"
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/client-emulator/internal/app"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.LoadGlobalConfig("services/client-emulator/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск Нагрузочного Тестера Абонентов WebRTC-Mesh Platform...")

	// Читаем специфичные для эмулятора адреса (для демонстрации захардкодим маппинг из конфига)
	sigAddr := "localhost:50055"
	chatAddr := "localhost:50057"

	// 1. Подключаемся к сигнализации (порт 50055)
	sigConn, err := grpc.Dial(sigAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Не удалось подключиться к signaling-gateway: %v", err)
	}
	defer sigConn.Close()
	sigClient := gen.NewMediaSignalingBridgeClient(sigConn)

	// 2. Подключаемся к аналитике чата (порт 50057)
	chatConn, err := grpc.Dial(chatAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Не удалось подключиться к chat-history-service: %v", err)
	}
	defer chatConn.Close()
	chatClient := gen.NewChatHistoryBridgeClient(chatConn)

	// 3. Собираем и запускаем эмулятор нагрузки
	emulator := app.NewLoadEmulator(
		log,
		sigClient,
		chatClient,
		50, // 50 параллельных горутин
		100*time.Millisecond,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Включаем штурм!
	emulator.StartSpawningBlast(ctx)

	// Взводим пустой сервер для прохождения Graceful Shutdown стандартов
	server := grpc.NewServer()
	listener, _ := net.Listen("tcp", cfg.BindAddr)
	go func() { _ = server.Serve(listener) }()

	shutdown.ListenSignals(log, server, 5*time.Second)
}
