package app

import (
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// HandleWsSignal терминирует Full-Duplex поток, коммутирует WebRTC SDP фреймы, команды модерации и векторные стрелки
func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	// 1. Атомарно регистрируем живую сессию участника в RAM-шарде за O(1)
	shard.mu.Lock()
	pConn := &PeerConnection{
		PeerID:      peerID,
		WS:          ws,
		IsModerator: isModerator,
		IsMuted:     false,
	}
	// Если пула коннектов для этой комнаты еще нет — аллоцируем память
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}
	shard.conns[roomID][peerID] = pConn

	// Если комнаты нет в карте структур — подстраховываем стейт
	if _, exists := shard.rooms[roomID]; !exists {
		shard.rooms[roomID] = &domain.VideoRoom{
			RoomID:    roomID,
			MaxPeers:  100,
			IsPaused:  false,
			Peers:     make(map[string]*domain.PeerSession),
			CreatedAt: time.Now(),
		}
		shard.lruCache.Set(roomID, shard.rooms[roomID])
	}

	shard.rooms[roomID].Peers[peerID] = &domain.PeerSession{
		PeerID:        peerID,
		IsModerator:   isModerator,
		IsMuted:       false,
		LastHeartbeat: time.Now(),
	}
	shard.mu.Unlock()

	s.log.Info("PEER ACTIVE -> Участник [%s] (Модератор: %v) успешно вошел в контур комнаты [%s]", peerID, isModerator, roomID)

	// 2. Graceful-зачистка сетевых сокетов при разрыве связи
	defer func() {
		shard.mu.Lock()
		delete(shard.conns[roomID], peerID)
		delete(shard.rooms[roomID].Peers, peerID)
		shard.mu.Unlock()
		_ = ws.Close()

		s.broadcastToRoom(roomID, map[string]any{
			"type":    "peer_left",
			"peer_id": peerID,
		})
	}()

	// Оповещаем комнату о входе нового участника для генерации WebRTC SDP Offer
	s.broadcastToRoom(roomID, map[string]any{
		"type":    "peer_joined",
		"peer_id": peerID,
	})

	// 3. Бесконечный цикл чтения WebSocket-фреймов
	for {
		var msg map[string]any
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {

		// ФИЧА №19 (ГОТОВО): Перехват векторных координат стрелок и веерная рассылка
		case "draw_vector":
			// Прокидываем координаты рисования Canvas всем остальным участникам в комнате
			s.broadcastToRoomExcept(roomID, peerID, msg)

		// Веерный проброс WebRTC метаданных (SDP Offer / Answer / ICE) по P2P Mesh-мосту
		case "sdp_offer", "sdp_answer", "ice_candidate":
			target, _ := msg["target_peer_id"].(string)
			s.sendToPeer(roomID, target, msg)

		// Обработка Управляющих Директив Модерации (Control Frames) (Req. 1)
		case "control_frame":
			if !isModerator {
				s.log.Error("[SECURITY ALERT] Попытка взлома модерации от Peer: %s", peerID)
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

		// Обработка сообщений живого чата с вызовом серверной санитизации
		case "chat_msg":
			rawText, _ := msg["text"].(string)

			// Вызываем наше Core-ядро безопасности и логера чата
			sanitizedText, _ := s.ProcessIncomingMessage(roomID, peerID, rawText)

			s.broadcastToRoom(roomID, map[string]any{
				"type":      "chat_broadcast",
				"sender_id": peerID,
				"text":      sanitizedText,
			})
		}
	}
}

// Вспомогательный метод веерной рассылки по комнате
func (s *SignalingService) broadcastToRoom(roomID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	for _, p := range shard.conns[roomID] {
		_ = p.WS.WriteJSON(msg)
	}
}

// Вспомогательный метод веерной рассылки всем, КРОМЕ отправителя (Идеально для рисования)
func (s *SignalingService) broadcastToRoomExcept(roomID, exceptPeerID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	for id, p := range shard.conns[roomID] {
		if id != exceptPeerID {
			_ = p.WS.WriteJSON(msg)
		}
	}
}

// Вспомогательный метод точечной доставки фрейма конкретному пиру
func (s *SignalingService) sendToPeer(roomID, peerID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if p, exists := shard.conns[roomID][peerID]; exists {
		_ = p.WS.WriteJSON(msg)
	}
}
