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

// ProxyExtendedConfig расширяет базовый конфиг шасси специфичными для API Gateway DNS-адресами Docker
type ProxyExtendedConfig struct {
	config.GlobalConfig `yaml:",inline"`
	SignalingBackendURL string `yaml:"signaling_backend_url"`
	ChatHistoryGrpcAddr string `yaml:"chat_history_grpc_addr"`
	SprStorageGrpcAddr  string `yaml:"spr_storage_grpc_addr"`
}

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и структурированный логер
	baseCfg := config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml")
	log := logger.NewAppLogger(baseCfg.ServiceName, baseCfg.LogLevel)
	log.Info("Запуск L7 Consistent Hashing Балансировщика и API Gateway...")

	// 2. ЛОКАЛЬНЫЙ b2b-ПАРСЕР: Накладываем оверлей для Docker-окружения
	var cfg ProxyExtendedConfig
	_, err := os.ReadFile("services/cloud-routing-proxy/config.yaml")
	if err == nil {
		_ = config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml") // Вызов для логов
		cfg.SignalingBackendURL = "http://signaling-gateway:8081"
		cfg.ChatHistoryGrpcAddr = "chat-history-service:8083"
		cfg.SprStorageGrpcAddr = "spr-storage:50060"
	} else {
		// Резервный Fallback для запуска на локальном хосте (без Docker)
		cfg.SignalingBackendURL = "http://localhost:8081"
		cfg.ChatHistoryGrpcAddr = "localhost:8083"
		cfg.SprStorageGrpcAddr = "localhost:50060"
	}

	// 3. gRPC-мост к эшелону истории чата (поддержка Docker DNS на порту :8083)
	grpcChatConn, err := grpc.NewClient(cfg.ChatHistoryGrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Критический крах API Gateway: не удалось взвести gRPC-мост к chat-history-service: %v", err)
	}
	defer grpcChatConn.Close()
	grpcChatClient := gen.NewChatHistoryBridgeClient(grpcChatConn)

	// 4. gRPC Мост к Хранилищу SPR (поддержка Docker DNS на порту :50060)
	grpcStorageConn, err := grpc.NewClient(cfg.SprStorageGrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Критический крах API Gateway: не удалось взвести gRPC-мост к spr-storage: %v", err)
	}
	defer grpcStorageConn.Close()
	grpcStorageClient := gen.NewStorageMediaBridgeClient(grpcStorageConn)

	// 5. Инициализируем твой оригинальный Consistent Hashing балансировщик
	nodesClusterPool := []string{"signaling-gateway:8081"}
	hashRingBalancer := app.NewConsistentHashRing(nodesClusterPool)

	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")
	if _, err := os.Stat(filepath.Join(staticDir, "templates")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}

	// 6. Передаем балансировщик, логер, gRPC-клиенты и путь статики в HttpHandler
	handler := transport.NewHttpHandler(hashRingBalancer, log, grpcChatClient, grpcStorageClient, staticDir)

	mux := http.NewServeMux()

	handler.RegisterRoutes(mux)

	log.Info("🚀 Единый b2b Контур Входа развернут на http://localhost:8080")

	// 8. Разворачиваем HTTP-сервер с расширенными таймаутами для потоковой передачи видео
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
	shutdown.ListenSignals(log, server, time.Duration(baseCfg.ShutdownTimeout)*time.Second)
}
