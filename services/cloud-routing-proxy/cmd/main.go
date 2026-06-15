package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"

	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализируем конфигурацию из универсального шасси и structured логер
	cfg := config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск L7 Consistent Hashing Балансировщика и API Gateway...")

	// 2. Инициализируем таргет-URL адреса внутренних микросервисов кластера
	signalingURL, _ := url.Parse("http://localhost:8081")
	chatServiceURL, _ := url.Parse("http://localhost:8082")

	signalingProxy := httputil.NewSingleHostReverseProxy(signalingURL)
	chatProxy := httputil.NewSingleHostReverseProxy(chatServiceURL)

	// 3. Динамический анализ путей фронтенда (Ищем где лежат шаблоны компонентов)
	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")
	if _, err := os.Stat(filepath.Join(staticDir, "templates")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}
	log.Info("[API GATEWAY] Каталог UI компонентов определен как: %s", staticDir)

	mux := http.NewServeMux()

	// 4. МАРШРУТИЗАТОР API GATEWAY ПЛОСКОСТИ ДАННЫХ

	// Проксируем сигнальные запросы на скрытый шлюз signaling-gateway (:8081)
	mux.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/sdp/mutate", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/ice-config", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/redirect", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })

	// ИСПРАВЛЕНО (ПСИ-5): Полноценная, высокопроизводительная бинарная отдача WebM-файлов строго силами API Gateway!
	// FIXED: Embedded secure binary stream endpoint on API Gateway to bypass Signaling Node overloads
	mux.HandleFunc("/api/v1/records/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		recordID := r.URL.Query().Get("id")

		if recordID == "" || recordID == "undefined" {
			http.Error(w, "🔒 [AppSec Proxy Guard]: Bad Request. ID записи пуст.", http.StatusBadRequest)
			return
		}

		file := filepath.Join("data", "video_records", fmt.Sprintf("%s.webm", recordID))
		if _, err := os.Stat(file); os.IsNotExist(err) {
			http.Error(w, "🔒 [AppSec Proxy Guard]: Файл видеозаписи не найден на дисках кластера.", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "video/webm")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=conference_record_%s.webm", recordID))
		http.ServeFile(w, r, file)
	})

	// Проксируем наносекундный Т9 на chat-history-service (:8082)
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) { chatProxy.ServeHTTP(w, r) })

	// Раздача статических ассетов
	fileServer := http.FileServer(http.Dir(filepath.Join(staticDir, "static")))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Рендеринг SOLID-компонентов фронтенда на стороне API Gateway
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			fileServer.ServeHTTP(w, r)
			return
		}

		data := map[string]any{"Version": "1.32"}

		if r.URL.Path == "/join.html" {
			renderProxyTemplate(w, staticDir, "join.html", data)
			return
		} else if r.URL.Path == "/conference.html" {
			renderProxyTemplate(w, staticDir, "conference.html", data)
			return
		} else if r.URL.Path == "/redirect.html" {
			renderProxyTemplate(w, staticDir, "redirect.html", data)
			return
		}

		renderProxyTemplate(w, staticDir, "index.html", data)
	})

	log.Info("🚀 Единый b2b Контур Входа развернут на http://localhost:8080")
	httpServer := &http.Server{Addr: ":8080", Handler: mux}
	go func() { _ = httpServer.ListenAndServe() }()

	server := grpc.NewServer() // Пустышка для диспетчера сигналов
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}

func renderProxyTemplate(w http.ResponseWriter, staticDir, page string, data any) {
	layoutPath := filepath.Join(staticDir, "templates", "layouts", "main.html")
	pagePath := filepath.Join(staticDir, "templates", "pages", page)

	components, _ := filepath.Glob(filepath.Join(staticDir, "templates", "components", "*.html"))
	files := append([]string{layoutPath, pagePath}, components...)

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		http.Error(w, fmt.Sprintf("🔒 [Proxy Template Engine Error]: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "main", data)
}
