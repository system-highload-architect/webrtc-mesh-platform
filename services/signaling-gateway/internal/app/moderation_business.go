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
			ChatHistory: make([]map[string]any, 0),
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

	// 1. Сначала выплевываем новому сокету исторический дамп чата (Ring Buffer)
	if len(shard.rooms[roomID].ChatHistory) > 0 {
		_ = ws.WriteJSON(map[string]any{
			"type": "chat_history_dump",
			"logs": shard.rooms[roomID].ChatHistory,
		})
	}

	// 2. ФИЧА (ГОТОВО): Собираем снапшот всех активных участников в RAM для ликвидации изоляции окон (Req. 3)
	// FIXED: Compile and dispatch instantaneous room snapshot dump for newly instantiated socket channels
	var existingPeers []string
	for existingID := range shard.rooms[roomID].Peers {
		if existingID != peerID { // Свой собственный ID в массив не включаем
			existingPeers = append(existingPeers, existingID)
		}
	}

	if len(existingPeers) > 0 {
		_ = ws.WriteJSON(map[string]any{
			"type":  "room_peers_snapshot",
			"peers": existingPeers,
		})
	}
	shard.mu.Unlock()

	s.log.Info("PEER ACTIVE -> Участник [%s] вошел в комнату [%s], получил дамп чата и снапшот %d пиров", peerID, roomID, len(existingPeers))

	defer func() {
		shard.mu.Lock()
		delete(shard.conns[roomID], peerID)
		delete(shard.rooms[roomID].Peers, peerID)
		shard.mu.Unlock()
		_ = ws.Close()

		if isModerator {
			s.StopServerRecording(roomID)
		}

		s.broadcastToRoom(roomID, map[string]any{"type": "peer_left", "peer_id": peerID})
	}()

	// Оповещаем остальных участников о входе новой ноды
	s.broadcastToRoomExcept(roomID, peerID, map[string]any{"type": "peer_joined", "peer_id": peerID})

	for {
		var msg map[string]any
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {

		case "server_record_control":
			if !isModerator {
				continue
			}
			cmd, _ := msg["command"].(string)
			if cmd == "START" {
				file, downloadLink := s.StartServerRecording(roomID)
				s.broadcastToRoom(roomID, map[string]any{"type": "chat_broadcast", "sender_id": "[СИСТЕМА]", "text": "🔴 ЗАПУЩЕНА ПЕРСИСТЕНТНАЯ СЕРВЕРНАЯ ЗАПИСЬ НА СТОРОНЕ NVMe БЭКЕНДА."})
				_ = ws.WriteJSON(map[string]any{"type": "record_started", "file": file, "link": downloadLink})
			} else if cmd == "STOP" {
				s.StopServerRecording(roomID)
				s.broadcastToRoom(roomID, map[string]any{"type": "chat_broadcast", "sender_id": "[СИСТЕМА]", "text": "💾 Серверная запись сессии успешно остановлена и сохранена в кластере компании."})
			}

		case "vad_ping":
			volume, _ := msg["volume"].(float64)
			if volume > 40.0 {
				shard.mu.Lock()
				room := shard.rooms[roomID]
				if room.ActiveSpeakerID != peerID {
					room.ActiveSpeakerID = peerID
					s.log.Info("[VAD TELEMETRY] Доминирующий спикер в комнате %s изменился на: %s", roomID, peerID)
					s.broadcastToRoom(roomID, map[string]any{
						"type":       "active_speaker_changed",
						"speaker_id": peerID,
					})
				}
				shard.mu.Unlock()
			}

		case "draw_vector":
			s.broadcastToRoomExcept(roomID, peerID, msg)

		case "sdp_offer", "sdp_answer":
			if sdpText, ok := msg["sdp"].(string); ok {
				s.WriteMediaFrame(roomID, sdpText)
			}
			target, _ := msg["target_peer_id"].(string)
			s.sendToPeer(roomID, target, msg)

		case "ice_candidate":
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
			case "RESUME_CONFERENCE":
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = false
				shard.mu.Unlock()
				s.broadcastToRoom(roomID, map[string]any{"type": "room_resumed"})
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

			shard.mu.Lock()
			shard.rooms[roomID].ChatHistory = append(shard.rooms[roomID].ChatHistory, chatFrame)
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
