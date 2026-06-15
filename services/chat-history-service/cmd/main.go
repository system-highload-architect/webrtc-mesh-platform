package main

import (
	"context"
	"net/http"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/services/chat-history-service/internal/app"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/chat-history-service/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск выделенного Data Plane сервиса чата chat-history-service...")

	// 2. Взводим Use-Case ядро Т9-движка и Layout Switcher раскладки клавиатуры
	chatEngine := app.NewChatHistoryEngine(log)

	mux := http.NewServeMux()

	// v1 REST Эндпоинт наносекундного поиска Т9 подсказок
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		prefix := r.URL.Query().Get("prefix")

		suggestion, found := chatEngine.QueryT9Prediction(context.Background(), prefix)
		if found {
			_, _ = w.Write([]byte(suggestion))
		} else {
			// Отдаем 200 OK с пустой строкой для идеальной тишины в консоли
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(""))
		}
	})

	// ИСПРАВЛЕНО: Явно форсируем прослушивание порта :8082, сопряженного с L7 прокси
	log.Info("🌐 REST-сервер chat-history-service успешно запущен на порту :8082")
	httpServer := &http.Server{Addr: ":8082", Handler: mux}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Крах HTTP-рантайма chat-history-service: %v", err)
		}
	}()

	server := grpc.NewServer() // Пустышка для диспетчера сигналов shutdown
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
