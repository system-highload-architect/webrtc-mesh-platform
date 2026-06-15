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
	transport "webrtc-mesh-platform/services/signaling-gateway/transport/grpc"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/signaling-gateway/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск WebSocket/gRPC Шлюза Сигнализации WebRTC...")

	// 2. Подключаемся к выделенному сервису чата для Т9-проксирования по gRPC мосту
	chatConn, err := grpc.Dial("localhost:50057", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Не удалось установить gRPC соединение с chat-history-service: %v", err)
	}
	defer chatConn.Close()
	chatClient := gen.NewChatHistoryBridgeClient(chatConn)

	// 3. Взводим декомпозированное Use-Case ядро комнат
	signalingCore := app.NewSignalingService(log)
	grpcHandler := transport.NewGrpcHandler(signalingCore)

	// 4. Запускаем бинарный gRPC сервер комнат
	server := grpc.NewServer()
	gen.RegisterMediaSignalingBridgeServer(server, grpcHandler)

	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}
	go func() { _ = server.Serve(listener) }()

	// 5. ВЗВОДИМ HTTP РУТИНГ И СТРОГОЕ ВЕРСИОНИРОВАНИЕ API V1
	mux := http.NewServeMux()

	// v1 Эндпоинт WebSocket Сигнализации с обязательной JWT-авторизацией (Req. 5)
	mux.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room")
		tokenStr := r.URL.Query().Get("token") // Принимаем криптографический токен личности

		if roomID == "" || tokenStr == "" {
			http.Error(w, "Missing room or security authorization token", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("[WS ERROR] Не удалось выполнить upgrade сокета: %v", err)
			return
		}

		// Передаем управление в moderation_business.go для валидации подписи подписи
		signalingCore.HandleWsSignal(roomID, tokenStr, conn)
	})

	// v1 Эндпоинт Проксирования Т9 подсказок из префиксного Trie-дерева чата
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		resp, err := chatClient.QueryT9Autocomplete(context.Background(), &gen.T9QueryRequest{Prefix: prefix})
		if err == nil && resp.IsFound {
			_, _ = w.Write([]byte(resp.Suggestion))
		}
	})

	// v1 Эндпоинт Проксирования и санитизации сообщений чата в асинхронную пакетную очередь
	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		room := r.URL.Query().Get("room")
		sender := r.URL.Query().Get("sender")
		text := r.URL.Query().Get("text")

		ack, err := chatClient.IngestChatMessage(context.Background(), &gen.ChatMessagePayload{
			RoomId: room, SenderId: sender, MessageText: text,
		})
		if err == nil {
			_, _ = w.Write([]byte(ack.SanitizedText))
		}
	})

	// Раздача статических ассетов (CSS, JS, Swagger) из изолированной папки web/static/
	fileServer := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Раздача корневого индексного файла интерфейса из web/
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

	log.Info("🌐 Интерактивный Web-интерфейс API v1 доступен по адресу: http://localhost:8081")
	httpServer := &http.Server{Addr: ":8081", Handler: mux}
	go func() { _ = httpServer.ListenAndServe() }()

	// 6. Передаем управление Graceful Shutdown диспетчеру
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
