package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/cloud-routing-proxy/internal/app"
	transport "webrtc-mesh-platform/services/cloud-routing-proxy/transport/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и структурированный логер
	cfg := config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск L7 Consistent Hashing Балансировщика и API Gateway...")

	// 2. gRPC-мост к эшелону истории чата (порт :8083)
	grpcChatConn, err := grpc.NewClient("localhost:8083", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Критический крах API Gateway: не удалось взвести gRPC-мост к chat-history-service: %v", err)
	}
	defer grpcChatConn.Close()
	grpcChatClient := gen.NewChatHistoryBridgeClient(grpcChatConn)

	// 3. gRPC Мост к Хранилищу SPR (порт :50060)
	grpcStorageConn, err := grpc.NewClient("localhost:50060", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Критический крах API Gateway: не удалось взвести gRPC-мост к spr-storage: %v", err)
	}
	defer grpcStorageConn.Close()
	grpcStorageClient := gen.NewStorageMediaBridgeClient(grpcStorageConn)

	// 4. Инициализируем твой оригинальный Consistent Hashing балансировщик
	nodesClusterPool := []string{"localhost:8081"}
	hashRingBalancer := app.NewConsistentHashRing(nodesClusterPool)

	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")
	if _, err := os.Stat(filepath.Join(staticDir, "templates")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}

	// 5. Инициализируем HttpHandler, передавая все бинарные зависимости кластера
	handler := transport.NewHttpHandler(hashRingBalancer, log, grpcChatClient, grpcStorageClient, staticDir)

	mux := http.NewServeMux()

	// 6. Нативно взводим все b2b-маршруты.
	// ИСПРАВЛЕНО (Уничтожение паники дублирования корня): Полностью выжгли ручной HandleFunc("/")!
	// Метод RegisterRoutes сам под капотом зарегистрирует и статику, и корень, и рендеринг страниц через твой handler.go!
	// FIXED: Removed duplicate inline fallback routes to completely eliminate duplicate pattern panics
	handler.RegisterRoutes(mux)

	log.Info("🚀 Единый b2b Контур Входа развернут на http://localhost:8080")

	// 7. Разворачиваем HTTP-сервер с расширенными таймаутами для потоковой передачи видео
	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadTimeout:       10 * time.Minute,
		WriteTimeout:      10 * time.Minute,
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Крах HTTP-рантайма центрального API Gateway: %v", err)
		}
	}()

	server := grpc.NewServer()
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
