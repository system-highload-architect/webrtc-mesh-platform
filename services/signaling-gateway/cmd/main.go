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
	"webrtc-mesh-platform/services/signaling-gateway/internal/app"
	grpcTransport "webrtc-mesh-platform/services/signaling-gateway/transport/grpc"
	httpTransport "webrtc-mesh-platform/services/signaling-gateway/transport/http"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/signaling-gateway/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск WebSocket/gRPC Шлюза Сигнализации WebRTC (Control Plane Mode)...")

	// 2. Взводим декомпозированное Use-Case ядро комнат, CAS лимитеров и NVMe рекордера
	signalingCore := app.NewSignalingService(log)

	// Инициализируем адаптеры gRPC и HTTP транспортов (Strict DI)
	grpcHandler := grpcTransport.NewGrpcHandler(signalingCore)
	httpHandler := httpTransport.NewHttpHandler(signalingCore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Запускаем фоновые b2b-воркеры (Пакетный логер чата, Каскадный LRU кэш, Экспоненциальный Бэкофф)
	signalingCore.StartChatBatchWorker(ctx)
	signalingCore.StartBackgroundJanitors(ctx)

	// 4. Запускаем бинарный gRPC сервер комнат для межсервисного общения
	server := grpc.NewServer()
	gen.RegisterMediaSignalingBridgeServer(server, grpcHandler)

	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}
	go func() { _ = server.Serve(listener) }()

	// 5. ВЗВОДИМ ЧИСТЫЙ МАРШРУТИЗАТОР СИГНАЛОВ API V1 (Только WebRTC сигналы!)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/ws", httpHandler.HandleWebSocket)
	mux.HandleFunc("/api/v1/chat/send", httpHandler.HandleChatSend)
	mux.HandleFunc("/api/v1/ice-config", httpHandler.HandleIceConfig)
	mux.HandleFunc("/api/v1/sdp/mutate", httpHandler.HandleSdpMutator)
	mux.HandleFunc("/api/v1/redirect", httpHandler.HandleSafeRedirect)

	log.Info("🌐 Изолированное ядро сигнализации готово принимать трафик на внутреннем порту: 8081")
	httpServer := &http.Server{Addr: ":8081", Handler: mux}
	go func() { _ = httpServer.ListenAndServe() }()

	// 6. Передаем управление Graceful Shutdown диспетчеру
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
