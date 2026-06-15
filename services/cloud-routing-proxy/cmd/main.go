package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/services/cloud-routing-proxy/internal/app"

	"google.golang.org/grpc"
)

func main() {
	cfg := config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск L7 Consistent Hashing Балансировщика cloud-routing-proxy...")

	// Захардкодим список нод из конфига для демонстрации
	nodes := []string{"localhost:8081"} // Перенаправляем на наш веб-шлюз
	ring := app.NewConsistentHashRing(nodes)

	mux := http.NewServeMux()

	// Умный динамический b2b-реверс-прокси
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room")
		if roomID == "" {
			roomID = "default_shared_room"
		}

		// Детерминированно вычисляем целевой сервер за O(log N)
		targetNode, err := ring.RouteRoom(roomID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		targetURL, _ := url.Parse("http://" + targetNode)
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// Перенаправляем L7 HTTP / WebSocket трафик на нужную ноду сигнализации
		r.Host = targetURL.Host
		proxy.ServeHTTP(w, r)
	})

	log.Info("🎛️ Балансировщик cloud-routing-proxy успешно поднят на порту %s", cfg.BindAddr)
	httpServer := &http.Server{Addr: cfg.BindAddr, Handler: mux}
	go func() { _ = httpServer.ListenAndServe() }()

	server := grpc.NewServer() // Пустышка для шатдауна
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
