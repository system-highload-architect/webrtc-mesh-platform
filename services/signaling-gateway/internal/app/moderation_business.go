package app

import (
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	pConn := &PeerConnection{
		PeerID:      peerID,
		WS:          ws,
		IsModerator: isModerator,
		IsMuted:     false,
	}
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}
	shard.conns[roomID][peerID] = pConn

	if _, exists := shard.rooms[roomID]; !exists {
		shard.rooms[roomID] = &domain.VideoRoom{
			RoomID:      roomID,
			MaxPeers:    100,
			IsPaused:    false,
			Peers:       make(map[string]*domain.PeerSession),
			ChatHistory: make([]map[string]any, 0), // Инициализируем слайс чата
			CreatedAt:   time.Now(),
		}
		shard.lruCache.Set(roomID, shard.rooms[roomID])
	}

	shard.rooms[roomID].Peers[peerID] = &domain.PeerSession{
		PeerID:        peerID,
		IsModerator:   isModerator,
		IsMuted:       false,
		LastHeartbeat: time.Now(),
	}

	// ФИЧА (ГОТОВО): Дамп истории чата новому подключившемуся пользователю
	// Если в комнате уже есть сохраненные логи, выстреливаем их персонально этому сокету
	if len(shard.rooms[roomID].ChatHistory) > 0 {
		_ = ws.WriteJSON(map[string]any{
			"type": "chat_history_dump",
			"logs": shard.rooms[roomID].ChatHistory,
		})
	}
	shard.mu.Unlock()

	s.log.Info("PEER ACTIVE -> Участник [%s] вошел в комнату [%s] и получил дамп истории чата", peerID, roomID)

	defer func() {
		shard.mu.Lock()
		delete(shard.conns[roomID], peerID)
		delete(shard.rooms[roomID].Peers, peerID)
		shard.mu.Unlock()
		_ = ws.Close()

		s.broadcastToRoom(roomID, map[string]any{"type": "peer_left", "peer_id": peerID})
	}()

	s.broadcastToRoom(roomID, map[string]any{"type": "peer_joined", "peer_id": peerID})

	for {
		var msg map[string]any
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {

		case "draw_vector":
			s.broadcastToRoomExcept(roomID, peerID, msg)

		case "sdp_offer", "sdp_answer", "ice_candidate":
			target, _ := msg["target_peer_id"].(string)
			s.sendToPeer(roomID, target, msg)

		case "control_frame":
			if !isModerator {
				continue
			}
			cmd, _ := msg["command"].(string)
			target, _ := msg["target_peer_id"].(string)

			switch cmd {
			case "SET_PAUSE":
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = true
				shard.mu.Unlock()
				s.broadcastToRoom(roomID, map[string]any{"type": "room_paused"})
			case "MUTE_AUDIO":
				s.sendToPeer(roomID, target, map[string]any{"type": "force_mute"})
			case "KICK_PEER":
				s.sendToPeer(roomID, target, map[string]any{"type": "force_kick"})
			}

		case "chat_msg":
			rawText, _ := msg["text"].(string)
			sanitizedText, _ := s.ProcessIncomingMessage(roomID, peerID, rawText)

			chatFrame := map[string]any{
				"type":      "chat_broadcast",
				"sender_id": peerID,
				"text":      sanitizedText,
			}

			// АТОМАРНОЕ НАКОПЛЕНИЕ ИСТОРИИ ЧАТА В RAM КОМНАТЫ
			shard.mu.Lock()
			shard.rooms[roomID].ChatHistory = append(shard.rooms[roomID].ChatHistory, chatFrame)
			// Удерживаем скользящее окно буфера в пределах 50 сообщений во избежание OOM
			if len(shard.rooms[roomID].ChatHistory) > 50 {
				shard.rooms[roomID].ChatHistory = shard.rooms[roomID].ChatHistory[1:]
			}
			shard.mu.Unlock()

			s.broadcastToRoom(roomID, chatFrame)
		}
	}
}

func (s *SignalingService) broadcastToRoom(roomID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	for _, p := range shard.conns[roomID] {
		_ = p.WS.WriteJSON(msg)
	}
}

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

func (s *SignalingService) sendToPeer(roomID, peerID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if p, exists := shard.conns[roomID][peerID]; exists {
		_ = p.WS.WriteJSON(msg)
	}
}
