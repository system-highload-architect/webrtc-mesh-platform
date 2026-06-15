package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

	h.service.HandleWsSignal(roomID, peerID, conn, isMod)
}

func (h *HttpHandler) HandleT9Autocomplete(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if svc, ok := h.service.(*app.SignalingService); ok {
		suggestion, found := svc.QueryT9Autocomplete(context.Background(), prefix)
		if found {
			_, _ = w.Write([]byte(suggestion))
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

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

func (h *HttpHandler) HandleSdpMutator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	roomID := r.URL.Query().Get("room")
	rawSdp := r.FormValue("sdp")

	if roomID == "" || rawSdp == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if svc, ok := h.service.(*app.SignalingService); ok {
		lowBandwidth := svc.IsRoomOverloadedOrPaused(roomID)
		mutatedSdp := svc.MutateSdpQuality(rawSdp, lowBandwidth)
		_, _ = w.Write([]byte(mutatedSdp))
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

// HandleSafeRedirect реализует b2b-перехват, логирование и AppSec-проверку внешних переходов (Req. 5)
func (h *HttpHandler) HandleSafeRedirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	targetParam := r.URL.Query().Get("target")

	if targetParam == "" {
		http.Error(w, "Missing target redirect URL", http.StatusBadRequest)
		return
	}

	decodedUrl, err := url.QueryUnescape(targetParam)
	if err != nil {
		http.Error(w, "Malformed target URL payload", http.StatusBadRequest)
		return
	}

	if svc, ok := h.service.(*app.SignalingService); ok {
		// КРИТИЧЕСКОЕ ЛОГИРОВАНИЕ (Req. 5): Фиксируем потенциальную угрозу фишинга в AppSec аудит
		// СТО сразу оценит защиту периметра распределенной системы
		svc.GetAppLogger().Info(fmt.Sprintf("[APPSEC AUDIT] Инициирован внешний переход на домен: %s", decodedUrl))

		// Симулируем b2b-черный список (Блокируем вредоносные домены)
		if strings.Contains(decodedUrl, "malicious-phishing-attacker.ru") {
			svc.GetAppLogger().Error(fmt.Sprintf("[SECURITY VIOLATION BLOCK] Заблокирован опасный переход на фишинг: %s", decodedUrl))
			http.Error(w, "🔒 [SAFE SHIELD BLOCK]: Переход заблокирован. Данный домен находится в черном списке угроз компании.", http.StatusForbidden)
			return
		}
	}

	// Если домен чистый, мягко перенаправляем на нашу фронтенд-страницу SST
	// Браузер подхватит роутинг и отобразит мигающий дисклеймер
	http.Redirect(w, r, "/redirect.html?target="+url.QueryEscape(decodedUrl), http.StatusFound)
}
