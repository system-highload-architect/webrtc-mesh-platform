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

// RegisterRoutes связывает эндпоинты с HTTP-мультиплексором
func (h *HttpHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ws", h.HandleWebSocketProxy)
	mux.HandleFunc("/api/v1/records/upload", h.HandleUploadRecord)
	mux.HandleFunc("/api/v1/records/download", h.HandleDownloadRecord)
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

	// ИСПРАВЛЕНО: Защита Favicon от паники 500
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

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

func (h *HttpHandler) HandleSafeTransfer(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	htmlPage := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="ru">
	<head><meta charset="UTF-8"><title>AppSec Security Shield</title></head>
	<body style="background:#020617;color:#fff;font-family:monospace;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;">
		<div style="background:#0b1329;padding:30px;border-radius:8px;border:1px solid #ef4444;text-align:center;max-width:500px;">
			<h2 style="color:#ef4444;">⚠️ ВНИМАНИЕ: ВНЕШНИЙ ПЕРЕХОД</h2>
			<p style="word-break:break-all;background:#020617;padding:10px;border-radius:4px;font-size:12px;border:1px solid #1e293b;">%s</p>
			<a href="%s" target="_blank" style="display:inline-block;margin-top:20px;background:#ef4444;color:#fff;padding:10px 20px;border-radius:4px;text-decoration:none;font-weight:bold;">Я осознаю риск, ПЕРЕЙТИ</a>
		</div>
	</body></html>`, targetURL, targetURL)
	_, _ = w.Write([]byte(htmlPage))
}

func (h *HttpHandler) HandleUploadRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	recordID := r.URL.Query().Get("id")
	dirPath := filepath.Join("data", "video_records")
	_ = os.MkdirAll(dirPath, 0755)
	filePath := filepath.Join(dirPath, fmt.Sprintf("%s.webm", recordID))
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	_, _ = io.Copy(file, r.Body)
	w.WriteHeader(http.StatusOK)
}

func (h *HttpHandler) HandleDownloadRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	recordID := r.URL.Query().Get("id")
	filePath := filepath.Join("data", "video_records", fmt.Sprintf("%s.webm", recordID))
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer file.Close()
	fileInfo, _ := file.Stat()
	w.Header().Set("Content-Type", "video/webm")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
}

// HandleRenderPage осуществляет каскадный поиск HTML-шаблонов
// ИСПРАВЛЕНО (Защита от 500 ошибок рендеринга): Сканируем папки web во всех возможных директориях запуска
// FIXED: Reengineered template parsing loop to locate static folders across project scopes safely
func (h *HttpHandler) HandleRenderPage(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/static/") {
		return
	}
	tokenStr := r.URL.Query().Get("token")
	isModerator := strings.Contains(tokenStr, "organizer")

	pageFile := "index.html"
	if r.URL.Path == "/join.html" {
		pageFile = "join.html"
	} else if r.URL.Path == "/conference.html" {
		pageFile = "conference.html"
	}

	// Алгоритм каскадного поиска папки web на диске Windows
	searchPaths := []string{
		h.staticDir,
		"web",
		"services/signaling-gateway/web",
		"../signaling-gateway/web",
		"../../services/signaling-gateway/web",
	}

	var baseStatic string
	var layoutPath string
	var pagePath string

	for _, p := range searchPaths {
		testLayout := filepath.Join(p, "templates", "layouts", "main.html")
		if _, err := os.Stat(testLayout); err == nil {
			baseStatic = p
			layoutPath = testLayout
			pagePath = filepath.Join(p, "templates", "pages", pageFile)
			break
		}
	}

	if baseStatic == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("🔒 [Proxy Template Error 500]: Директория 'web/templates' не найдена. Запустите 'make run' из корня проекта."))
		return
	}

	files := []string{layoutPath, pagePath}
	components, _ := filepath.Glob(filepath.Join(baseStatic, "templates", "components", "*.html"))
	if len(components) > 0 {
		files = append(files, components...)
	}

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf("🔒 [Proxy Template Parse Error]: %v", err)))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{"Version": "1.38", "IsModerator": isModerator, "RoomID": "clearway_pki_session"}
	_ = tmpl.ExecuteTemplate(w, "main", data)
}
