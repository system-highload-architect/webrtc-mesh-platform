package main

import (
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/spr-storage/internal/app"
	transport "webrtc-mesh-platform/services/spr-storage/transport/grpc"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и структурированный логер
	cfg := config.LoadGlobalConfig("services/spr-storage/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск gRPC эмулятора распределенной базы данных ScyllaDB spr-storage...")

	// 2. Взводим Use-Case ядро NoSQL базы данных и gRPC-рекордера
	storageService := app.NewSprStorageService(log)

	// 3. Открываем единый внутренний gRPC TCP-порт :50060 внутри Docker-сети
	bindAddr := "0.0.0.0:50060"
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatal("Критический крах: не удалось открыть внутренний gRPC-порт базы %s: %v", bindAddr, err)
	}

	// 4. Создаем высокопроизводительный промышленный gRPC-сервер
	grpcServer := grpc.NewServer()

	// 5. Инициализируем твой оригинальный GrpcHandler для обслуживания профилей из ScyllaDB
	// Передаем туда storageService (он на 100% реализует абстрактный интерфейс app.SprStorageEngine)
	identityHandler := transport.NewGrpcHandler(storageService)

	// 6. СЛИЯНИЕ КОНТУРОВ (Регистрация двух контрактов на одном порту :50060):
	// Регистрируем первый контракт — управление b2b-паспортами пользователей (auth.proto)
	gen.RegisterAuthenticationBridgeServer(grpcServer, identityHandler)

	// Регистрируем второй контракт — бинарный потоковый рекордер WebM-кадров (storage.proto)
	// FIXED: Bound double-plane domain servers over the single gRPC network port listener socket
	gen.RegisterStorageMediaBridgeServer(grpcServer, storageService)

	go func() {
		log.Info("📡 gRPC-сервер базы spr-storage успешно запущен на порту :50060")
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера spr-storage: %v", err)
		}
	}()

	// 7. Передаем gRPC-сервер в диспетчер сигналов для безопасного Graceful Shutdown
	shutdown.ListenSignals(log, grpcServer, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
