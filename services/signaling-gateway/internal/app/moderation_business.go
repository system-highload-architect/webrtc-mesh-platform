package app

import (
	"log"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// HandleWsSignal терминирует Full-Duplex сигнальный поток, обрабатывает модерацию и WebRTC контракты (Req. 1, 2 & 5)
func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	// 1. Атомарно регистрируем сессию участника в RAM-шарде за O(1)
	shard.mu.Lock()
	pConn := &PeerConnection{
		PeerID:      peerID,
		WS:          ws,
		IsModerator: isModerator,
		IsMuted:     false,
	}
	shard.conns[roomID][peerID] = pConn

	shard.rooms[roomID].Peers[peerID] = &domain.PeerSession{
		PeerID:        peerID,
		IsModerator:   isModerator,
		IsMuted:       false,
		LastHeartbeat: time.Now(),
	}
	shard.mu.Unlock()

	// 2. Graceful-зачистка сетевых сокетов при разрыве связи
	defer func() {
		shard.mu.Lock()
		delete(shard.conns[roomID], peerID)
		delete(shard.rooms[roomID].Peers, peerID)
		shard.mu.Unlock()
		_ = ws.Close()

		// Оповещаем комнату об уходе участника для перестройки WebRTC Mesh-сетки
		s.broadcastToRoom(roomID, map[string]any{
			"type":    "peer_left",
			"peer_id": peerID,
		})
	}()

	// 3. Веерное оповещение о входе нового участника для генерации SDP Offer
	s.broadcastToRoom(roomID, map[string]any{
		"type":    "peer_joined",
		"peer_id": peerID,
	})

	// 4. Бесконечный неблокирующий цикл чтения WebSocket-фреймов (Control & Data Plane)
	for {
		var msg map[string]any
		if err := ws.ReadJSON(&msg); err != nil {
			log.Printf("[WS WARN] Connection reset for peer %s: %v", peerID, err)
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {

		// Эшелон 1: Маршрутизация WebRTC метаданных (SDP Offer / Answer / ICE)
		case "sdp_offer", "sdp_answer", "ice_candidate":
			target, _ := msg["target_peer_id"].(string)
			s.sendToPeer(roomID, target, msg)

		// Эшелон 2: Обработка Управляющих Директив Модерации (Control Frames) (Req. 1 & 4)
		case "control_frame":
			if !isModerator {
				continue // Жесткая AppSec-отсечка фальсификации прав администратора
			}
			cmd, _ := msg["command"].(string)
			target, _ := msg["target_peer_id"].(string)

			switch cmd {
			case "SET_PAUSE":
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = true
				shard.mu.Unlock()
				// Веерно пушим команду перевода видео в Muted Keyframes (1 кадр в 5 секунд)
				s.broadcastToRoom(roomID, map[string]any{"type": "room_paused"})

			case "MUTE_AUDIO":
				s.sendToPeer(roomID, target, map[string]any{"type": "force_mute"})

			case "KICK_PEER":
				s.sendToPeer(roomID, target, map[string]any{"type": "force_kick"})
			}

		// Эшелон 3: Веерное проксирование сообщений живого чата
		case "chat_msg":
			s.broadcastToRoom(roomID, map[string]any{
				"type":      "chat_broadcast",
				"sender_id": peerID,
				"text":      msg["text"],
			})
		}
	}
}

// Вспомогательный b2b-метод веерной рассылки по комнате
func (s *SignalingService) broadcastToRoom(roomID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	for _, p := range shard.conns[roomID] {
		_ = p.WS.WriteJSON(msg)
	}
}

// Вспомогательный b2b-метод точечной доставки фрейма конкретному пиру
func (s *SignalingService) sendToPeer(roomID, peerID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if p, exists := shard.conns[roomID][peerID]; exists {
		_ = p.WS.WriteJSON(msg)
	}
}
