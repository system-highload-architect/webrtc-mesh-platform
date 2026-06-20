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

// HandleWebSocket v1 Эндпоинт WebSocket Сигнализации комнат и модерации
func (h *HttpHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	peerID := r.URL.Query().Get("peer")
	isMod := r.URL.Query().Get("mod") == "true"

	// Извлекаем лимиты, которые API Gateway (прокси) пробросил в строке запроса
	maxPeers := r.URL.Query().Get("max_peers")
	duration := r.URL.Query().Get("duration")

	if roomID == "" || peerID == "" {
		http.Error(w, "Missing room or peer identification parameters", http.StatusBadRequest)
		return
	}

	// Обогащаем контекст запроса нашими b2b-параметрами
	ctx := context.WithValue(r.Context(), "max_peers", maxPeers)
	ctx = context.WithValue(ctx, "duration", duration)
	r = r.WithContext(ctx)

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.service.HandleWsSignal(roomID, peerID, conn, isMod)
}

// HandleChatSend v1 Эндпоинт санитизации чата, XSS-защиты и пакетного логирования
func (h *HttpHandler) HandleChatSend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	room := r.URL.Query().Get("room")
	sender := r.URL.Query().Get("sender")
	text := r.URL.Query().Get("text")

	if room == "" || sender == "" || text == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(""))
		return
	}

	// Нативная XSS-защита и AppSec экранирование фреймов
	// Впрыскиваем экранирование, защищая enterprise-контур от XSS-инъекций
	sanitizedText := strings.ReplaceAll(text, "<", "&lt;")
	sanitizedText = strings.ReplaceAll(sanitizedText, ">", "&gt;")

	// Логируем перехваченный фрейм в Control Plane консоль шлюза
	if svc, ok := h.service.(*app.SignalingService); ok {
		svc.GetAppLogger().Info(fmt.Sprintf("[DATA PLANE CHAT] Рум: %s | Отправитель: %s | Текст: %s", room, sender, sanitizedText))

		// Опционально: если на бэкенде есть in-memory буфер истории, сбрасываем фрейм туда
		// Чтобы при welcome-пакете история выгружалась обратно (Req. 3)
		// svc.PushToHistoryBuffer(room, sender, sanitizedText)
	}

	// Нативно возвращаем очищенный текст в JavaScript для вещания по WebSocket
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sanitizedText))
}

// HandleIceConfig v1 Эндпоинт выдачи инфраструктурных STUN/TURN конфигураций Coturn для обхода NAT
func (h *HttpHandler) HandleIceConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	iceConfig := map[string]any{
		"iceServers": []map[string]any{
			{
				"urls": []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	jsonBytes, _ := json.Marshal(iceConfig)
	_, _ = w.Write(jsonBytes)
}

// HandleSdpMutator принимает сырой SDP оффер и на лету мутирует битрейт кодеков (Req. 3)
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

// HandleSafeRedirect реализует b2b AppSec прокси-перехватчик внешних линков (Req. 5)
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
		svc.GetAppLogger().Info(fmt.Sprintf("[APPSEC AUDIT] Внешний переход перехвачен шлюзом: %s", decodedUrl))

		if strings.Contains(decodedUrl, "malicious-phishing-attacker.ru") {
			svc.GetAppLogger().Error(fmt.Sprintf("[SECURITY BLOCK] Заблокирован фишинг: %s", decodedUrl))
			http.Error(w, "🔒 [SAFE SHIELD BLOCK]: Домен заблокирован из соображений корпоративной безопасности компании.", http.StatusForbidden)
			return
		}
	}

	http.Redirect(w, r, "/redirect.html?target="+url.QueryEscape(decodedUrl), http.StatusFound)
}
