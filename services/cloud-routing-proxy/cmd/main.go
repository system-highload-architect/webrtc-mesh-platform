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
	cfg := config.LoadGlobalConfig("services/cloud-routing-proxy/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск L7 Consistent Hashing Балансировщика и API Gateway...")

	signalingURL, _ := url.Parse("http://localhost:8081")
	chatServiceURL, _ := url.Parse("http://localhost:8082")

	signalingProxy := httputil.NewSingleHostReverseProxy(signalingURL)
	chatProxy := httputil.NewSingleHostReverseProxy(chatServiceURL)

	basePath, _ := os.Getwd()
	staticDir := filepath.Join(basePath, "web")
	if _, err := os.Stat(filepath.Join(staticDir, "meet.html")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}
	log.Info("[API GATEWAY] Каталог UI ассетов определен как: %s", staticDir)

	mux := http.NewServeMux()

	// МАРШРУТИЗАЦИЯ КЛАСТЕРА НА УРОВНЕ L7 REVERSE PROXY
	mux.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/sdp/mutate", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/ice-config", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/redirect", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) { chatProxy.ServeHTTP(w, r) })

	// Прямая отдача WebM-видеофайлов с NVMe силами прокси
	mux.HandleFunc("/api/v1/records/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		recordID := r.URL.Query().Get("id")
		if recordID == "" || recordID == "undefined" {
			http.Error(w, "🔒 [AppSec Proxy Guard]: ID записи пуст.", http.StatusBadRequest)
			return
		}
		file := filepath.Join("data", "video_records", fmt.Sprintf("%s.webm", recordID))
		if _, err := os.Stat(file); os.IsNotExist(err) {
			http.Error(w, "🔒 [AppSec Proxy Guard]: Файл записи не найден.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "video/webm")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=conference_record_%s.webm", recordID))
		http.ServeFile(w, r, file)
	})

	// Раздача статики
	fileServer := http.FileServer(http.Dir(filepath.Join(staticDir, "static")))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// ВРЕМЕННО: Отрезаем падения из-за отсутствия HTML. Отдаем статус заглушки для проверки ручек бэкенда поштучно
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			fileServer.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "API Gateway Online. Ready for backend endpoints validation."}`))
	})

	log.Info("🚀 Единый b2b Контур Входа развернут на http://localhost:8080")
	httpServer := &http.Server{Addr: ":8080", Handler: mux}
	go func() { _ = httpServer.ListenAndServe() }()

	server := grpc.NewServer()
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}

func renderMeetTemplate(w http.ResponseWriter, staticDir string, data any) {
	meetPath := filepath.Join(staticDir, "meet.html")
	components, _ := filepath.Glob(filepath.Join(staticDir, "templates", "components", "*.html"))

	files := append([]string{meetPath}, components...)
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		http.Error(w, fmt.Sprintf("🔒 [Proxy Template Engine Error]: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}
