package http

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

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/services/cloud-routing-proxy/internal/app"
)

type HttpHandler struct {
	balancer  app.BalancerEngine
	log       *logger.AppLogger
	staticDir string
}

func NewHttpHandler(balancer app.BalancerEngine, log *logger.AppLogger, staticDir string) *HttpHandler {
	return &HttpHandler{
		balancer:  balancer,
		log:       log,
		staticDir: staticDir,
	}
}

func (h *HttpHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ws", h.HandleWebSocketProxy)
	mux.HandleFunc("/api/v1/records/upload", h.HandleUploadRecord)
	mux.HandleFunc("/api/v1/records/download", h.HandleDownloadRecord)

	// ИСПРАВЛЕНО (Safe Transfer Page ТЗ): Регистрируем ручку промежуточного экрана безопасности ссылок
	mux.HandleFunc("/safe-transfer", h.HandleSafeTransfer)

	defaultSignalingURL, _ := url.Parse("http://localhost:8081")
	signalingProxy := httputil.NewSingleHostReverseProxy(defaultSignalingURL)

	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/sdp/mutate", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/ice-config", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/redirect", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })

	chatServiceURL, _ := url.Parse("http://localhost:8082")
	chatProxy := httputil.NewSingleHostReverseProxy(chatServiceURL)
	mux.HandleFunc("/api/v1/t9", func(w http.ResponseWriter, r *http.Request) { chatProxy.ServeHTTP(w, r) })

	fileServer := http.FileServer(http.Dir(filepath.Join(h.staticDir, "static")))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/", h.HandleRenderPage)
}

func (h *HttpHandler) HandleWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		http.Error(w, "Bad Request: Missing room ID parameter", http.StatusBadRequest)
		return
	}

	targetAddr, err := h.balancer.RouteRoom(roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://%s", targetAddr))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	h.log.Info("🎯 [L7 ROUTER] Consistent Hashing сопряжение. Комната [%s] приземлена на узел [%s]", roomID, targetAddr)
	proxy.ServeHTTP(w, r)
}

// ИСПРАВЛЕНО (Реализация Safe Transfer Page): Рендерим глухой WAF-экран предупреждения перед переходом
// FIXED: Provisioned dynamic HTML safe guard layout page intercepting unsafe outbound link redirections
func (h *HttpHandler) HandleSafeTransfer(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	htmlPage := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="ru">
	<head>
		<meta charset="UTF-8">
		<title>AppSec Security Shield</title>
		<style>
			body { background: #020617; color: #f8fafc; font-family: monospace; display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; margin: 0; }
			.card { background: #0b1329; border: 1px solid #ef4444; padding: 30px; border-radius: 8px; max-width: 500px; text-align: center; box-shadow: 0 0 20px rgba(239, 68, 68, 0.2); }
			a { color: #3b82f6; text-decoration: underline; font-weight: bold; }
			.btn { display: inline-block; margin-top: 20px; background: #ef4444; color: #fff; padding: 10px 20px; border-radius: 4px; text-decoration: none; font-weight: bold; }
		</style>
	</head>
	<body>
		<div class="card">
			<h2 style="color: #ef4444;">⚠️ ВНИМАНИЕ: ВНЕШНИЙ ПЕРЕХОД</h2>
			<p style="color: #8b949e; line-height: 1.6;">Вы покидаете защищенный корпоративный PKI контур платформы. Администрация не несет ответственности за содержимое внешнего ресурса.</p>
			<p style="word-break: break-all; background: #020617; padding: 10px; border-radius: 4px; font-size: 12px; border: 1px solid #1e293b;">%s</p>
			<a href="%s" target="_blank" class="btn">Я осознаю риск, ПЕРЕЙТИ</a>
		</div>
	</body>
	</html>`, targetURL, targetURL)

	_, _ = w.Write([]byte(htmlPage))
}

func (h *HttpHandler) HandleUploadRecord(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, 500*1024*1024)

	dirPath := filepath.Join("data", "video_records")
	_ = os.MkdirAll(dirPath, 0755)

	filePath := filepath.Join(dirPath, fmt.Sprintf("%s.webm", recordID))
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *HttpHandler) HandleDownloadRecord(w http.ResponseWriter, r *http.Request) {
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
}

func (h *HttpHandler) HandleRenderPage(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/static/") {
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

	layoutPath := filepath.Join(h.staticDir, "templates", "layouts", "main.html")
	pagePath := filepath.Join(h.staticDir, "templates", "pages", pageFile)
	components, _ := filepath.Glob(filepath.Join(h.staticDir, "templates", "components", "*.html"))
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
