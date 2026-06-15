package main

import (
	"context"
	"encoding/json"
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
	transport "webrtc-mesh-platform/services/signaling-gateway/transport/grpc"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/signaling-gateway/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск WebSocket/gRPC Шлюза Сигнализации WebRTC...")

	// 2. Взводим декомпозированное Use-Case ядро комнат, Т9-движка и логера чата
	signalingCore := app.NewSignalingService(log)
	grpcHandler := transport.NewGrpcHandler(signalingCore)

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
	// Находим абсолютный путь запуска процесса для точной привязки папки web
	// ИСПРАВЛЕНО: Убираем дублирование папки "static" при инициализации файлового сервера
	// FIXED: Resolved path mirroring to prevent style.css 404 compilation drops
	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")

	if _, err := os.Stat(filepath.Join(staticDir, "index.html")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}
	log.Info("Паттерн деплоя -> Корневой каталог ассетов определен как: %s", staticDir)

	mux := http.NewServeMux()

	// v1 Эндпоинт WebSocket Сигнализации комнат
	mux.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room")
		peerID := r.URL.Query().Get("peer")
		isMod := r.URL.Query().Get("mod") == "true"

		if roomID == "" || peerID == "" {
			http.Error(w, "Missing room or peer identification parameters", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("[WS ERROR] Не удалось выполнить upgrade сокета: %v", err)
			return
		}

		signalingCore.HandleWsSignal(roomID, peerID, conn, isMod)
	})

	// v1 Эндпоинт выдачи инфраструктурных STUN/TURN конфигураций Coturn для обхода NAT
	mux.HandleFunc("/api/v1/ice-config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		iceConfig := signalingCore.FetchIceServersConfig()
		jsonBytes, _ := json.Marshal(iceConfig)
		_, _ = w.Write(jsonBytes)
	})

	// v1 Эндпоинт Прямого наносекундного поиска Т9 подсказок
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		suggestion, found := signalingCore.QueryT9Autocomplete(context.Background(), prefix)
		if found {
			_, _ = w.Write([]byte(suggestion))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// v1 Эндпоинт Нативной санитизации чата, XSS-защиты
	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		room := r.URL.Query().Get("room")
		sender := r.URL.Query().Get("sender")
		text := r.URL.Query().Get("text")

		sanitizedText, _ := signalingCore.ProcessIncomingMessage(room, sender, text)
		_, _ = w.Write([]byte(sanitizedText))
	})

	// ИСПРАВЛЕНО: Файловый сервер смотрит строго на корень папки web, а префикс /static/ корректно отсекается
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", fileServer)

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
