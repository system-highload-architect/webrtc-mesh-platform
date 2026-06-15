package main

import (
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config" // НАШЕ ЕДИНОЕ ПЛАТФОРМЕННОЕ ШАССИ КОНФИГУРАЦИИ
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/spr-storage/internal/app"
	transport "webrtc-mesh-platform/services/spr-storage/transport/grpc"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем локальный контур конфигурации из общего шасси и structured логер
	cfg := config.LoadGlobalConfig("services/spr-storage/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск gRPC Эмулятора Базы Профилей ScyllaDB spr-storage...")

	// 2. Взводим декомпозированное Use-Case ядро NoSQL базы данных через интерфейсный контракт (Strict DI)
	var sprCore app.SprStorageEngine = app.NewSprStorageService(log)

	// 3. Собираем бинарный gRPC-транспорт адаптер
	grpcHandler := transport.NewGrpcHandler(sprCore)

	server := grpc.NewServer()
	gen.RegisterAuthenticationBridgeServer(server, grpcHandler)

	// 4. Открываем сетевой сокет на прослушивание выделенного b2b-порта :50060
	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт базы %s: %v", cfg.BindAddr, err)
	}

	go func() {
		log.Info("gRPC-сервер ScyllaDB (SPR) успешно запущен на %s", cfg.BindAddr)
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера spr-storage: %v", err)
		}
	}()

	// 5. Плавная остановка (Graceful Shutdown) для зачистки сетевых сокетов операционной системы
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
