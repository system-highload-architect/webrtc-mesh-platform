package http

import (
	"context"
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

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/pb/gen" // Подключаем твои оригинальные сгенерированные gRPC-контракты чата
	"webrtc-mesh-platform/services/cloud-routing-proxy/internal/app"
)

type HttpHandler struct {
	balancer          app.BalancerEngine
	log               *logger.AppLogger
	grpcChatClient    gen.ChatHistoryBridgeClient
	grpcStorageClient gen.StorageMediaBridgeClient // ИСПРАВЛЕНО: Инжектируем бинарный gRPC-клиент хранилища
	staticDir         string
}

func NewHttpHandler(
	balancer app.BalancerEngine,
	log *logger.AppLogger,
	chatClient gen.ChatHistoryBridgeClient,
	storageClient gen.StorageMediaBridgeClient, // Передаем клиент в конструктор
	staticDir string,
) *HttpHandler {
	return &HttpHandler{
		balancer:          balancer,
		log:               log,
		grpcChatClient:    chatClient,
		grpcStorageClient: storageClient,
		staticDir:         staticDir,
	}
}

// RegisterRoutes связывает эндпоинты с HTTP-мультиплексором
func (h *HttpHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ws", h.HandleWebSocketProxy)
	mux.HandleFunc("/api/v1/records/upload", h.HandleUploadRecord)
	mux.HandleFunc("/api/v1/records/download", h.HandleRecordsDownload)
	mux.HandleFunc("/safe-transfer", h.HandleSafeTransfer)

	defaultSignalingURL, _ := url.Parse("http://signaling-gateway:8081")
	signalingProxy := httputil.NewSingleHostReverseProxy(defaultSignalingURL)

	mux.HandleFunc("/api/v1/chat/send", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/sdp/mutate", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/ice-config", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })
	mux.HandleFunc("/api/v1/redirect", func(w http.ResponseWriter, r *http.Request) { signalingProxy.ServeHTTP(w, r) })

	mux.HandleFunc("/api/v1/t9", h.HandleT9Autocomplete)

	// Защита Favicon от паники 500
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	fileServer := http.FileServer(http.Dir(filepath.Join(h.staticDir, "static")))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/", h.HandleRenderPage)
}

// HandleT9Autocomplete — НАШ АДАПТЕР: Принимает HTTP от браузера, конвертирует в бинарный T9QueryRequest
// и отправляет по внутреннему gRPC-каналу в chat-history-service на порт :8083
// FIXED: Transformed raw HTTP request contexts to compile with generated QueryT9Autocomplete gRPC specs
func (h *HttpHandler) HandleT9Autocomplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Жесткий enterprise SLA таймаут наносекундной предикции в 300мс
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	// Нативно вызываем твой ОРИГИНАЛЬНЫЙ gRPC-метод по Protobuf контракту
	grpcResponse, err := h.grpcChatClient.QueryT9Autocomplete(ctx, &gen.T9QueryRequest{Prefix: prefix})
	if err != nil || !grpcResponse.IsFound {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(grpcResponse.Suggestion))
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	recordID := r.URL.Query().Get("id")
	if recordID == "" || recordID == "undefined" {
		http.Error(w, "Bad Request: Missing ID", http.StatusBadRequest)
		return
	}

	// Выставляем b2b AppSec лимит на загрузку тяжелых монолитов WebM (500 МБ)
	r.Body = http.MaxBytesReader(w, r.Body, 500*1024*1024)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Нативно открываем бинарный gRPC-стрим к микросервису spr-storage по контракту storage.proto
	grpcStream, err := h.grpcStorageClient.StreamMediaChunk(ctx)
	if err != nil {
		h.log.Error("Не удалось инициализировать gRPC-стрим к spr-storage: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Выделяем b2b-буфер в 64 Килобайта для нарезки потока на gRPC-кадры (Chunking)
	buffer := make([]byte, 64*1024)
	for {
		bytesRead, readErr := r.Body.Read(buffer)
		if bytesRead > 0 {
			// Упаковываем сырые байты в строго типизированную Protobuf-структуру MediaChunkRequest
			sendErr := grpcStream.Send(&gen.MediaChunkRequest{
				RecordId: recordID,
				Data:     buffer[:bytesRead],
			})
			if sendErr != nil {
				h.log.Error("Крах передачи кадра по gRPC-стриму: %v", sendErr)
				http.Error(w, "Storage Streaming Failed", http.StatusInternalServerError)
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			http.Error(w, "HTTP Read Error", http.StatusInternalServerError)
			return
		}
	}

	// Запечатываем gRPC-стрим и забираем финальный бинарный статус ответа от базы ScyllaDB/SPR
	grpcResponse, err := grpcStream.CloseAndRecv()
	if err != nil {
		h.log.Error("[API GATEWAY] База данных SPR отклонила закрытие видео-стрима: %v", err)
		http.Error(w, "Database Persist Error", http.StatusInternalServerError)
		return
	}

	h.log.Info("🎯 [API GATEWAY] Финальный статус укладки WebM на NVMe диски SPR: [%s]", grpcResponse.Status)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("UPLOAD_SUCCESS"))
}

// HandleRecordsDownload транслирует медиа-файлы из NoSQL персистентного каталога
func (h *HttpHandler) HandleRecordsDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	recordID := r.URL.Query().Get("id")
	if recordID == "" || recordID == "undefined" {
		http.Error(w, "🔒 [AppSec Proxy Guard]: ID записи пуст.", http.StatusBadRequest)
		return
	}

	fileName := fmt.Sprintf("%s.webm", recordID)

	// Список b2b-путей для каскадного поиска файла внутри Docker-тома
	searchPaths := []string{
		filepath.Join("data", "scylladb_spr_emulation", "records", fileName),  // С подпапкой records
		filepath.Join("data", "scylladb_spr_emulation", fileName),             // Напрямую в корне тома
		filepath.Join("/", "app", "data", "scylladb_spr_emulation", fileName), // Абсолютный путь Docker
	}

	var file *os.File
	var err error

	// Перебираем пути, пока не найдем физический файл на NVMe-массиве
	for _, path := range searchPaths {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	// Если файл не найден ни по одному пути, выдаем безопасный AppSec 404
	if err != nil {
		h.log.Error("🗑️ [DOWNLOAD ERROR] Файл записи %s не найден ни по одному из b2b-путей.", recordID)
		http.Error(w, "🔒 [AppSec Proxy Guard]: Файл записи не найден в ScyllaDB/SPR keyspace.", http.StatusNotFound)
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

	// Разблокируем побайтовую Range-Streaming перемотку видео
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
}

func (h *HttpHandler) HandleRenderPage(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	isModerator := strings.Contains(tokenStr, "organizer")

	pageFile := "index.html"
	if r.URL.Path == "/join.html" {
		pageFile = "join.html"
	} else if r.URL.Path == "/conference.html" {
		pageFile = "conference.html"
	}

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
