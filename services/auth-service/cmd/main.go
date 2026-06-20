package main

import (
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config" // Платформенное шасси конфигурации
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/auth-service/internal/app"
	transport "webrtc-mesh-platform/services/auth-service/transport/grpc"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем локальный контур конфигурации из общего шасси и structured логер
	cfg := config.LoadGlobalConfig("services/auth-service/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск b2b gRPC-сервиса авторизации абонентов и генерации JWT...")

	// 2. Взводим декомпозированное Use-Case ядро через интерфейсный контракт (Strict DI)
	var authCore app.AuthUseCaseManager = app.NewAuthService(log)

	// 3. Собираем gRPC-транспорт адаптер
	grpcHandler := transport.NewGrpcHandler(authCore)

	server := grpc.NewServer()
	gen.RegisterAuthenticationBridgeServer(server, grpcHandler)

	// 4. Открываем сетевой сокет на прослушивание порта :50059
	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}

	go func() {
		log.Info("gRPC Authentication сервер успешно запущен на %s", cfg.BindAddr)
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера авторизации: %v", err)
		}
	}()

	// 5. Плавная остановка (Graceful Shutdown) для зачистки сетевых сокетов
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
