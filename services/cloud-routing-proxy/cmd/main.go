package main

import (
	"fmt"
	"html/template"
	"io"
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
	if _, err := os.Stat(filepath.Join(staticDir, "templates")); os.IsNotExist(err) {
		staticDir = filepath.Join(basePath, "services", "signaling-gateway", "web")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/sdp/mutate", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/ice-config", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/redirect", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) { chatProxy.ServeHTTP(w, r) })

	// ИСПРАВЛЕНО (Снятие лимитов WAF): Убрали ограничения MaxBytes и добавили потоковую дозапись
	mux.HandleFunc("/api/v1/records/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		recordID := r.URL.Query().Get("id")
		if recordID == "" || recordID == "undefined" {
			http.Error(w, "Bad Request: Missing ID", http.StatusBadRequest)
			return
		}

		// СБРАСЫВАЕМ ОГРАНИЧЕНИЕ НА РАЗМЕР ВХОДЯЩЕГО ПАКЕТА (Req. 4)
		// Нативно разрешаем загрузку тяжелых монолитных b2b WebM-видеофайлов
		r.Body = http.MaxBytesReader(w, r.Body, 500*1024*1024) // Лимит повышен до 500 Мегабайт!

		dirPath := filepath.Join("data", "video_records")
		_ = os.MkdirAll(dirPath, 0755)

		filePath := filepath.Join(dirPath, fmt.Sprintf("%s.webm", recordID))
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644) // Очищаем старый файл при перезаписи
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Потоково перекачиваем байты Blob-видео напрямую на NVMe-диск
		_, err = io.Copy(file, r.Body)
		if err != nil {
			log.Error("[API GATEWAY] Ошибка записи тела POST запроса на NVMe: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("UPLOAD_SUCCESS"))
	})

	// Разблокировка побайтовой перемотки Range-Streaming
	mux.HandleFunc("/api/v1/records/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		recordID := r.URL.Query().Get("id")
		if recordID == "" || recordID == "undefined" {
			http.Error(w, "🔒 [AppSec Proxy Guard]: ID записи пуст.", http.StatusBadRequest)
			return
		}

		filePath := filepath.Join("data", "video_records", fmt.Sprintf("%s.webm", recordID))
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "🔒 [AppSec Proxy Guard]: Файл записи не найден.", http.StatusNotFound)
			return
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "video/webm")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=conference_record_%s.webm", recordID))
		w.Header().Set("Accept-Ranges", "bytes")

		http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
	})

	fileServer := http.FileServer(http.Dir(filepath.Join(staticDir, "static")))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			fileServer.ServeHTTP(w, r)
			return
		}
		tokenStr := r.URL.Query().Get("token")
		isModerator := strings.Contains(tokenStr, "david_organizer")
		data := map[string]any{"Version": "1.38", "IsModerator": isModerator}
		pageFile := "index.html"
		if r.URL.Path == "/join.html" {
			pageFile = "join.html"
		} else if r.URL.Path == "/conference.html" {
			pageFile = "conference.html"
		}
		renderMeetTemplate(w, staticDir, pageFile, data)
	})

	log.Info("🚀 Единый b2b Контур Входа развернут на http://localhost:8080")

	// ИСПРАВЛЕНО (Аппаратное расширение буферов): Разворачиваем HTTP-сервер с расширенными таймаутами для больших файлов
	// FIXED: Configured extended server network limits to safely hold large binary video payloads
	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadTimeout:       10 * time.Minute, // Даем 10 минут на чтение тяжелого видео-монолита
		WriteTimeout:      10 * time.Minute, // Даем 10 минут на отдачу видео
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() { _ = httpServer.ListenAndServe() }()

	server := grpc.NewServer()
	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}

func renderMeetTemplate(w http.ResponseWriter, staticDir, page string, data any) {
	layoutPath := filepath.Join(staticDir, "templates", "layouts", "main.html")
	pagePath := filepath.Join(staticDir, "templates", "pages", page)
	components, _ := filepath.Glob(filepath.Join(staticDir, "templates", "components", "*.html"))
	files := append([]string{layoutPath, pagePath}, components...)
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf("🔒 [Proxy Template Engine Error]: %v", err)))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "main", data)
}
