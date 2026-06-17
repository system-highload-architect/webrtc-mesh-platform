package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/services/cloud-routing-proxy/internal/app"
	transport "webrtc-mesh-platform/services/cloud-routing-proxy/transport/http"

	"google.golang.org/grpc"
)

func main() {
	cfg := config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск L7 Consistent Hashing Балансировщика и API Gateway...")

	// 1. ИСПРАВЛЕНО: Инжектируем массив бэкендов в твой оригинальный конструктор кольца хэширования
	// FIXED: Dispatched signaling instances slice configuration into custom factory constructor
	signalingNodes := []string{"localhost:8081"}
	var balancer app.BalancerEngine = app.NewConsistentHashRing(signalingNodes)

	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")
	if _, err := os.Stat(filepath.Join(staticDir, "templates")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}

	// 2. Инициализируем транспортный HTTP эшелон и регистрируем чистые маршруты слоев
	httpHandler := transport.NewHttpHandler(balancer, log, staticDir)
	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	log.Info("🚀 Единый b2b Контур Входа развернут на http://localhost:8080")
	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadTimeout:       10 * time.Minute,
		WriteTimeout:      10 * time.Minute,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() { _ = httpServer.ListenAndServe() }()

	server := grpc.NewServer()
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
