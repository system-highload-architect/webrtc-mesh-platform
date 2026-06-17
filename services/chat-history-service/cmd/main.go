package main

import (
	"context"
	"net"
	"net/http"
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
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/chat-history-service/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск выделенного Data Plane сервиса чата chat-history-service...")

	// 2. Взводим Use-Case ядро боевой бизнес-логики HistoryService
	var historyService app.ChatHistoryProcessor = app.NewHistoryService(log)

	// Запускаем асинхронный фоновый воркер пакетного сброса логов переписки на NVMe-диск
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	historyService.StartBatchJanitor(ctx)

	mux := http.NewServeMux()

	// v1 REST Эндпоинт наносекундного поиска Т9 подсказок
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		prefix := r.URL.Query().Get("prefix")

		// Опрашиваем наше боевое ядро HistoryService через GetT9Suggestion контракт
		suggestion, found := historyService.GetT9Suggestion(context.Background(), prefix)
		if found {
			_, _ = w.Write([]byte(suggestion))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(""))
		}
	})

	// Явно форсируем прослушивание порта :8082, сопряженного с L7 прокси балансировщика
	log.Info("🌐 REST-сервер chat-history-service успешно запущен на порту :8082")
	httpServer := &http.Server{Addr: ":8082", Handler: mux}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Крах HTTP-рантайма chat-history-service: %v", err)
		}
	}()

	// ИСПРАВЛЕНО (Активация gRPC-порта чата): Открываем выделенный TCP-слушатель для шлюза сигнализации
	// FIXED: Bound high-performance gRPC private listener context on node address port :9082
	grpcBindAddr := "0.0.0.0:9082"
	grpcListener, err := net.Listen("tcp", grpcBindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть внутренний gRPC порт %s для межабонентского обмена: %v", grpcBindAddr, err)
	}

	server := grpc.NewServer()
	grpcHandler := transport.NewGrpcHandler(historyService)

	// Регистрируем наш обработчик в скомпилированный Protobuf-мост кластера
	gen.RegisterChatHistoryBridgeServer(server, grpcHandler)

	log.Info("📡 gRPC-сервер истории чата успешно развернут на порту :9082")
	go func() {
		if err := server.Serve(grpcListener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера chat-history-service: %v", err)
		}
	}()

	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
