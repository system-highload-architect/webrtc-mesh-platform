package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

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

	var activeRecordFile *os.File

	defer func() {
		if activeRecordFile != nil {
			_ = activeRecordFile.Close()
		}
		shard.mu.Lock()
		delete(shard.conns[roomID], peerID)
		if room, exists := shard.rooms[roomID]; exists {
			delete(room.Peers, peerID)
		}
		shard.mu.Unlock()
		_ = ws.Close()
		s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "peer-left", SenderID: peerID})
	}()

	for {
		// Возвращаем пуленепробиваемый ReadJSON
		var incoming domain.WsSession
		if err := ws.ReadJSON(&incoming); err != nil {
			break
		}

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

		if incoming.Type == "chat" {
			shard.mu.Lock()
			room := shard.rooms[roomID]
			room.ChatHistory = append(room.ChatHistory, map[string]any{"sender_id": peerID, "text": incoming.Text, "timestamp": time.Now()})
			shard.mu.Unlock()

			s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "chat_broadcast", SenderID: peerID, Text: incoming.Text})
			continue
		}

		if incoming.Type == "control_frame" {
			if incoming.Command == "SET_PAUSE" {
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = true
				shard.mu.Unlock()
				s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "room_paused"})
				continue
			} else if incoming.Command == "RESUME_CONFERENCE" {
				shard.mu.Lock()
				shard.rooms[roomID].IsPaused = false
				shard.mu.Unlock()
				s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "room_resumed"})
				continue
			}

			if incoming.Command == "MUTE_AUDIO" && incoming.TargetPeerID != "" {
				s.sendToPeerRaw(roomID, incoming.TargetPeerID, domain.WsSession{Type: "force_mute"})
				continue
			}

			if incoming.Command == "KICK_PEER" && incoming.TargetPeerID != "" {
				s.sendToPeerRaw(roomID, incoming.TargetPeerID, domain.WsSession{Type: "force_kick"})
				continue
			}

			if incoming.Command == "START_RECORD" {
				currentActiveRecordID := fmt.Sprintf("rec_%d", time.Now().Unix())
				s.log.Info("[REC ENGINE] Открытие NVMe-файла записи. ID: %s", currentActiveRecordID)

				dirPath := filepath.Join("data", "video_records")
				_ = os.MkdirAll(dirPath, 0755)

				filePath := filepath.Join(dirPath, fmt.Sprintf("%s.webm", currentActiveRecordID))

				if activeRecordFile != nil {
					_ = activeRecordFile.Close()
				}

				var err error
				activeRecordFile, err = os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					_, _ = activeRecordFile.Write([]byte{0x1A, 0x45, 0xDF, 0xA3})
				}

				_ = ws.WriteJSON(map[string]any{
					"type": "record_started",
					"file": currentActiveRecordID,
				})
				continue
			}

			if incoming.Command == "STOP_RECORD" {
				s.log.Info("[REC ENGINE] Серверный файл записи запечатан на диске.")
				if activeRecordFile != nil {
					_ = activeRecordFile.Close()
					activeRecordFile = nil
				}
				continue
			}
			continue
		}

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
