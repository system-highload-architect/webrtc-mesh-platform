package main

import (
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config" // ПОДКЛЮЧАЕМ ЕДИНОЕ ШАССИ КОМПАНИИ
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/signaling-gateway/internal/app"
	transport "webrtc-mesh-platform/services/signaling-gateway/transport/grpc"

	"google.golang.org/grpc"
)

func main() {
	// Инициализируем конфигурацию из универсального шасси
	cfg := config.LoadGlobalConfig("services/signaling-gateway/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Инициализация WebSocket/gRPC Шлюза Сигнализации Комнат WebRTC...")

	var signalingCore app.RoomManagerEngine = app.NewSignalingService()
	grpcHandler := transport.NewGrpcHandler(signalingCore)

	server := grpc.NewServer()
	gen.RegisterMediaSignalingBridgeServer(server, grpcHandler)

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

	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
