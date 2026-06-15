package http

import (
	"context"
	"encoding/json"
	"net/http"

	"webrtc-mesh-platform/services/signaling-gateway/internal/app"

	"github.com/gorilla/websocket"
)

type HttpHandler struct {
	service  app.RoomManagerEngine
	upgrader websocket.Upgrader
}

func NewHttpHandler(service app.RoomManagerEngine) *HttpHandler {
	return &HttpHandler{
		service: service,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// HandleWebSocket v1 Эндпоинт WebSocket Сигнализации комнат, модерации и P2P-векторных стрелок Canvas
func (h *HttpHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	peerID := r.URL.Query().Get("peer")
	isMod := r.URL.Query().Get("mod") == "true"

	if roomID == "" || peerID == "" {
		http.Error(w, "Missing room or peer identification parameters", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Нативно прокидываем коннект в слой декомпозированной модерации
	h.service.HandleWsSignal(roomID, peerID, conn, isMod)
}

// HandleT9Autocomplete v1 Эндпоинт Прямого наносекундного поиска Т9 подсказок
func (h *HttpHandler) HandleT9Autocomplete(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Прямой вызов нашего наносекундного Trie-дерева через метод расширения
	// Для поддержки обратной совместимости, если метод не объявлен в интерфейсе, сделаем явное приведение типов
	if svc, ok := h.service.(*app.SignalingService); ok {
		suggestion, found := svc.QueryT9Autocomplete(context.Background(), prefix)
		if found {
			_, _ = w.Write([]byte(suggestion))
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// HandleChatSend v1 Эндпоинт Нативной санитизации чата, XSS-защиты и пакетного логирования
func (h *HttpHandler) HandleChatSend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	room := r.URL.Query().Get("room")
	sender := r.URL.Query().Get("sender")
	text := r.URL.Query().Get("text")

	if svc, ok := h.service.(*app.SignalingService); ok {
		sanitizedText, _ := svc.ProcessIncomingMessage(room, sender, text)
		_, _ = w.Write([]byte(sanitizedText))
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

// HandleIceConfig v1 Эндпоинт выдачи инфраструктурных STUN/TURN конфигураций Coturn для обхода NAT
func (h *HttpHandler) HandleIceConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if svc, ok := h.service.(*app.SignalingService); ok {
		iceConfig := svc.FetchIceServersConfig()
		jsonBytes, _ := json.Marshal(iceConfig)
		_, _ = w.Write(jsonBytes)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
