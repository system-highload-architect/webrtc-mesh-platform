package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/signaling-gateway/internal/app"
	grpcTransport "webrtc-mesh-platform/services/signaling-gateway/transport/grpc"
	httpTransport "webrtc-mesh-platform/services/signaling-gateway/transport/http"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/signaling-gateway/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск WebSocket/gRPC Шлюза Сигнализации WebRTC...")

	// 2. Взводим декомпозированное Use-Case ядро комнат, Т9-движка и логера чата
	signalingCore := app.NewSignalingService(log)

	// Инициализируем адаптеры транспортов (Strict DI)
	grpcHandler := grpcTransport.NewGrpcHandler(signalingCore)
	httpHandler := httpTransport.NewHttpHandler(signalingCore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Запускаем фоновые b2b-воркеры (Пакетный логер чата, Каскадный LRU кэш, Экспоненциальный Бэкофф)
	signalingCore.StartChatBatchWorker(ctx)
	signalingCore.StartBackgroundJanitors(ctx)

	// 4. Запускаем бинарный gRPC сервер комнат
	server := grpc.NewServer()
	gen.RegisterMediaSignalingBridgeServer(server, grpcHandler)

	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}
	go func() { _ = server.Serve(listener) }()

	// 5. ДИНАМИЧЕСКИЙ АНАЛИЗ ПУТЕЙ СТАТИКИ (Ликвидация 404 ошибок верстки)
	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")
	if _, err := os.Stat(filepath.Join(staticDir, "index.html")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}
	log.Info("Паттерн деплоя -> Корневой каталог ассетов определен как: %s", staticDir)

	mux := http.NewServeMux()

	// ВЕРСИОНИРОВАНИЕ API V1 (Чистая b2b-маршрутизация без inline кода!)
	mux.HandleFunc("/api/v1/ws", httpHandler.HandleWebSocket)
	mux.HandleFunc("/api/v1/t9", httpHandler.HandleT9Autocomplete)
	mux.HandleFunc("/api/v1/chat/send", httpHandler.HandleChatSend)
	mux.HandleFunc("/api/v1/ice-config", httpHandler.HandleIceConfig)

	// Раздача статических ассетов через абсолютные пути
	fileServer := http.FileServer(http.Dir(filepath.Join(staticDir, "static")))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Раздача страниц Multi-Page роутинга
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		targetFile := filepath.Join(staticDir, "index.html")
		if r.URL.Path == "/join.html" {
			targetFile = filepath.Join(staticDir, "join.html")
		} else if r.URL.Path == "/conference.html" {
			targetFile = filepath.Join(staticDir, "conference.html")
		} else if r.URL.Path == "/redirect.html" {
			targetFile = filepath.Join(staticDir, "redirect.html")
		}
		http.ServeFile(w, r, targetFile)
	})

	log.Info("🌐 Интерактивный Web-интерфейс API v1 доступен по адресу: http://localhost:8081")
	httpServer := &http.Server{Addr: ":8081", Handler: mux}
	go func() { _ = httpServer.ListenAndServe() }()

	// 6. Передаем управление Graceful Shutdown диспетчеру
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
