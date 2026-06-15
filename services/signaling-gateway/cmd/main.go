package main

import (
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/signaling-gateway/internal/app"
	transport "webrtc-mesh-platform/services/signaling-gateway/transport/grpc"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем локальный контур конфигурации и structured логер
	cfg := config.LoadConfig("services/signaling-gateway/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Инициализация WebSocket/gRPC Шлюза Сигнализации Комнат WebRTC...")

	// 2. Взводим Use-Case слой ядра (32-way Map Sharding + Trie T9 Engine)
	var signalingCore app.RoomManagerEngine = app.NewSignalingService()

	// 3. Собираем gRPC-транспорт, прокидывая локальный доменный сервис
	grpcHandler := transport.NewGrpcHandler(signalingCore)

	server := grpc.NewServer()
	gen.RegisterMediaSignalingBridgeServer(server, grpcHandler)

	// 4. Открываем сетевой сокет на прослушивание порта :50055
	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}

	go func() {
		log.Info("gRPC WebRTC-Signaling сервер успешно запущен на %s", cfg.BindAddr)
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера сигнализации: %v", err)
		}
	}()

	// 5. Передаем управление Graceful Shutdown диспетчеру компании
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
