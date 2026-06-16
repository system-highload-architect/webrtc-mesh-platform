package app

import (
	"fmt"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// HandleWsSignal терминирует Full-Mesh WebRTC сигналы в RAM-шардах кластера
func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	pConn := &PeerConnection{PeerID: peerID, WS: ws, IsModerator: isModerator, IsMuted: false}
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}
	shard.conns[roomID][peerID] = pConn

	if _, exists := shard.rooms[roomID]; !exists {
		shard.rooms[roomID] = &domain.VideoRoom{
			RoomID: roomID, MaxPeers: 100, IsPaused: false,
			Peers: make(map[string]*domain.PeerSession), ChatHistory: make([]map[string]any, 0), CreatedAt: time.Now(),
		}
		shard.lruCache.Set(roomID, shard.rooms[roomID])
	}
	shard.mu.Unlock()

	defer func() {
		shard.mu.Lock()
		delete(shard.conns[roomID], peerID)
		if room, exists := shard.rooms[roomID]; exists {
			delete(room.Peers, peerID)
		}
		shard.mu.Unlock()
		_ = ws.Close()

		s.log.Info("[CONTROL PLANE] Абонент [%s] разорвал P2P-туннель комнаты [%s]", peerID, roomID)
		s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "peer-left", SenderID: peerID})
	}()

	for {
		var incoming domain.WsSession
		if err := ws.ReadJSON(&incoming); err != nil {
			break
		}

		// 1. Стартовая регистрация ноды в Mesh
		if incoming.Type == "join" {
			shard.mu.Lock()
			room := shard.rooms[roomID]
			room.Peers[peerID] = &domain.PeerSession{PeerID: peerID, IsModerator: isModerator, IsMuted: false, LastHeartbeat: time.Now()}

			var currentParticipants []map[string]string
			for id := range shard.conns[roomID] {
				if id != peerID {
					currentParticipants = append(currentParticipants, map[string]string{"id": id, "name": id})
				}
			}

			var historyLogs []map[string]string
			for _, h := range room.ChatHistory {
				historyLogs = append(historyLogs, map[string]string{
					"sender_id": h["sender_id"].(string), "text": h["text"].(string),
				})
			}
			shard.mu.Unlock()

			_ = ws.WriteJSON(map[string]any{"type": "welcome", "sender_id": peerID, "participants": currentParticipants})
			_ = ws.WriteJSON(map[string]any{"type": "chat_history_dump", "logs": historyLogs})

			s.broadcastToRoomExceptRaw(roomID, peerID, domain.WsSession{Type: "peer-joined", SenderID: peerID, SenderName: peerID})
			s.log.Info("[CONTROL PLANE] Абонент [%s] успешно зарегистрирован в RAM-комнате [%s]", peerID, roomID)
			continue
		}

		// 2. Бродкаст текстовых фреймов чата
		if incoming.Type == "chat" {
			shard.mu.Lock()
			room := shard.rooms[roomID]
			room.ChatHistory = append(room.ChatHistory, map[string]any{"sender_id": peerID, "text": incoming.Text, "timestamp": time.Now()})
			shard.mu.Unlock()

			s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "chat_broadcast", SenderID: peerID, Text: incoming.Text})
			continue
		}

		// 3. Перехват плоских управляющих директив модерации Давида
		if incoming.Type == "control_frame" {
			if incoming.Command == "SET_PAUSE" {
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = true
				shard.mu.Unlock()
				s.log.Info("[ORCHESTRATION] Модератор Давид зафиксировал Паузу на Бэкенде.")
				s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "room_paused"})
				continue
			} else if incoming.Command == "RESUME_CONFERENCE" {
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = false
				shard.mu.Unlock()
				s.log.Info("[ORCHESTRATION] Модератор Давид снял Паузу на Бэкенде.")
				s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "room_resumed"})
				continue
			}

			if incoming.Command == "MUTE_AUDIO" && incoming.TargetPeerID != "" {
				s.log.Error("[ORCHESTRATION] Бэкенд шлюза шлет force_mute на ноду [%s]", incoming.TargetPeerID)
				s.sendToPeerRaw(roomID, incoming.TargetPeerID, domain.WsSession{Type: "force_mute"})
				continue
			}

			if incoming.Command == "KICK_PEER" && incoming.TargetPeerID != "" {
				s.log.Error("[ORCHESTRATION] Бэкенд шлюза шлет force_kick на ноду [%s]", incoming.TargetPeerID)
				s.sendToPeerRaw(roomID, incoming.TargetPeerID, domain.WsSession{Type: "force_kick"})
				continue
			}

			if incoming.Command == "START_RECORD" {
				s.log.Info("[REC ENGINE] Модератор Давид запустил серверную запись NVMe...")
				// Имитируем или триггерим создание медиафайла на диске
				mockFileID := fmt.Sprintf("rec_%d.webm", time.Now().Unix())
				_ = ws.WriteJSON(map[string]any{
					"type": "record_started",
					"file": mockFileID,
				})
				continue
			}

			if incoming.Command == "STOP_RECORD" {
				s.log.Info("[REC ENGINE] Модератор Давид остановил серверную запись. Файл запечатан.")
				continue
			}

			continue
		}

		// 4. МАРШРУТИЗАЦИЯ СИГНАЛОВ НА ЦЕЛЕВОЙ TARGET ID (Offers / Answers / Candidates)
		// Теперь Payload нативно пересылается байт-в-байт удаленным браузерам!
		if incoming.TargetID != "" {
			incoming.SenderID = peerID
			s.sendToPeerRaw(roomID, incoming.TargetID, incoming)
		}
	}
}

func (s *SignalingService) broadcastToRoomRaw(roomID string, msg domain.WsSession) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	for _, p := range shard.conns[roomID] {
		_ = p.WS.WriteJSON(msg)
	}
}

func (s *SignalingService) broadcastToRoomExceptRaw(roomID, exceptPeerID string, msg domain.WsSession) {
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

func (s *SignalingService) sendToPeerRaw(roomID, peerID string, msg domain.WsSession) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if p, exists := shard.conns[roomID][peerID]; exists {
		_ = p.WS.WriteJSON(msg)
	}
}
