package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// Диспетчер команд Давида: Паттерн распределения режимов по кодам стейта
var roomCommandRegistry = map[string]struct {
	StateCode int
	OnType    string
	OffType   string
}{
	"GLOBAL_MUTE_AUDIO": {StateCode: 1, OnType: "force_mute_audio_lock", OffType: "force_unmute_audio_lock"},
	"GLOBAL_MUTE_VIDEO": {StateCode: 2, OnType: "force_mute_video_lock", OffType: "force_unmute_video_lock"},
	"SET_PAUSE":         {StateCode: 0, OnType: "room_paused", OffType: "room_resumed"},
}

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
			RoomID:      roomID,
			MaxPeers:    100,
			IsPaused:    false,
			Peers:       make(map[string]*domain.PeerSession),
			ChatHistory: make([]map[string]any, 0),
			CreatedAt:   time.Now(),
			RoomStates:  make(map[int]bool),
		}
		shard.lruCache.Set(roomID, shard.rooms[roomID])
	}

	room := shard.rooms[roomID]
	if room.RoomStates == nil {
		room.RoomStates = make(map[int]bool)
	}

	roomExists := false
	hasActiveModerator := false
	if room != nil {
		roomExists = true
		for id := range shard.conns[roomID] {
			if shard.conns[roomID][id] != nil && shard.conns[roomID][id].IsModerator {
				hasActiveModerator = true
				break
			}
		}
	}

	if !isModerator && (!roomExists || !hasActiveModerator) {
		shard.mu.Unlock()
		_ = ws.WriteJSON(map[string]string{
			"type": "waiting_for_moderator",
			"text": "Конференция еще не началась. Ожидайте авторизации Владельца комнаты...",
		})

		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				shard.mu.Lock()
				delete(shard.conns[roomID], peerID)
				shard.mu.Unlock()
				_ = ws.Close()
				return
			}
		}
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

		s.broadcastMapToRoom(roomID, map[string]string{
			"type":      "peer-left",
			"sender_id": peerID,
		})
	}()

	if isModerator {
		s.log.Info("👑 [CONTROL PLANE] Владелец Давид в сети! Активация сопряжения комнат [%s]", roomID)
		s.broadcastMapToRoom(roomID, map[string]string{
			"type": "room_activated",
		})
	}

	for {
		messageType, rawMessageBytes, err := ws.ReadMessage()
		if err != nil {
			break
		}

		if messageType == websocket.BinaryMessage {
			continue
		}

		var incoming domain.WsSession
		if err := json.Unmarshal(rawMessageBytes, &incoming); err != nil {
			continue
		}

		if incoming.Type == "record_chunk" {
			if activeRecordFile != nil && incoming.MediaBase64 != "" {
				decodedBytes, err := base64.StdEncoding.DecodeString(incoming.MediaBase64)
				if err == nil {
					_, _ = activeRecordFile.Write(decodedBytes)
				}
			}
			continue
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

			// Капкан Давида для защиты от F5 на хэш-карте кодов состояний комнаты
			shard.mu.RLock()
			if room != nil && room.RoomStates != nil {
				// Если зашедший НЕ создатель и НЕ текущий сохраненный Спикер — накрываем локами
				if !isModerator && peerID != room.CurrentSpeakerID {
					if room.RoomStates[1] {
						_ = ws.WriteJSON(map[string]string{"type": "force_mute_audio_lock"})
					}
					if room.RoomStates[2] {
						_ = ws.WriteJSON(map[string]string{"type": "force_mute_video_lock"})
					}
				}
				if room.RoomStates[0] {
					_ = ws.WriteJSON(map[string]string{"type": "room_paused"})
				}
			}
			shard.mu.RUnlock()

			s.broadcastMapToRoomExcept(roomID, peerID, map[string]string{
				"type":        "peer-joined",
				"sender_id":   peerID,
				"sender_name": peerID,
				"peer_id":     peerID,
			})
			continue
		}

		if incoming.Type == "chat" {
			shard.mu.Lock()
			if shard.rooms[roomID] != nil {
				shard.rooms[roomID].ChatHistory = append(shard.rooms[roomID].ChatHistory, map[string]any{"sender_id": peerID, "text": incoming.Text, "timestamp": time.Now()})
			}
			shard.mu.Unlock()

			s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "chat_broadcast", SenderID: peerID, Text: incoming.Text})
			continue
		}

		if incoming.Type == "control_frame" {
			// Сохраняем и сбрасываем ID Спикера прямо в оперативной памяти VideoRoom структуры бэкенда
			if incoming.Command == "SET_SPEAKER" && incoming.TargetPeerID != "" {
				shard.mu.Lock()
				if shard.rooms[roomID] != nil {
					shard.rooms[roomID].CurrentSpeakerID = incoming.TargetPeerID
				}
				shard.mu.Unlock()
				s.broadcastMapToRoom(roomID, map[string]string{"type": "focus_speaker", "target_peer_id": incoming.TargetPeerID})
				continue
			}
			if incoming.Command == "RESET_SPEAKER" {
				shard.mu.Lock()
				if shard.rooms[roomID] != nil {
					shard.rooms[roomID].CurrentSpeakerID = ""
				}
				shard.mu.Unlock()
				s.broadcastMapToRoom(roomID, map[string]string{"type": "reset_speaker"})
				continue
			}

			// Промышленный распределитель лекционных блокировок пассивного зала за O(1) без каскадов if-else
			if config, exists := roomCommandRegistry[incoming.Command]; exists {
				shard.mu.Lock()
				room := shard.rooms[roomID]
				if room.RoomStates == nil {
					room.RoomStates = make(map[int]bool)
				}

				room.RoomStates[config.StateCode] = !room.RoomStates[config.StateCode]
				isActiveNow := room.RoomStates[config.StateCode]

				if config.StateCode == 0 {
					room.IsPaused = isActiveNow
				}

				activeSpeakerID := ""
				if room != nil {
					activeSpeakerID = room.CurrentSpeakerID
				}
				shard.mu.Unlock()

				broadcastType := config.OffType
				if isActiveNow {
					broadcastType = config.OnType
				}

				s.log.Info("🎯 [DISPATCHER] Режим лекции команды [%s]. Код [%d] ➔ Статус: [%t]", incoming.Command, config.StateCode, isActiveNow)

				shard.mu.RLock()
				if shard.conns[roomID] != nil {
					for targetID, peerConn := range shard.conns[roomID] {
						// ИСПРАВЛЕНО (Иммунитет Организатора и Спикера по флагам):
						// Блокировка полностью ИГНОРИРУЕТ Создателя (p.IsModerator) и назначенного Спикера (activeSpeakerID)
						// FIXED: Screened out moderator and active speaker nodes from room wide lock signals
						if (peerConn != nil && peerConn.IsModerator) || targetID == activeSpeakerID || targetID == "David_Moderator" {
							continue
						}
						if peerConn != nil && peerConn.WS != nil {
							_ = peerConn.WS.WriteJSON(map[string]string{
								"type": broadcastType,
							})
						}
					}
				}
				shard.mu.RUnlock()
				continue
			}

			if incoming.Command == "RESUME_CONFERENCE" {
				shard.mu.Lock()
				if shard.rooms[roomID] != nil {
					shard.rooms[roomID].RoomStates[0] = false
					shard.rooms[roomID].IsPaused = false
				}
				shard.mu.Unlock()
				s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "room_resumed"})
				continue
			}

			// Точечные директивы мьюта и кика с окон видеоплиток — бьют без иммунитетов!
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
				_ = ws.WriteJSON(map[string]any{"type": "record_started", "file": currentActiveRecordID})
				continue
			}

			if incoming.Command == "STOP_RECORD" {
				s.log.Info("[REC ENGINE] Серверный файл записи запечатан.")
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

func (s *SignalingService) broadcastMapToRoom(roomID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if shard.conns[roomID] == nil || len(shard.conns[roomID]) == 0 {
		return
	}
	for _, p := range shard.conns[roomID] {
		if p != nil && p.WS != nil {
			_ = p.WS.WriteJSON(msg)
		}
	}
}

func (s *SignalingService) broadcastMapToRoomExcept(roomID string, exceptPeerID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if shard.conns[roomID] == nil || len(shard.conns[roomID]) <= 1 {
		return
	}
	for id, p := range shard.conns[roomID] {
		if id != exceptPeerID && p != nil && p.WS != nil {
			_ = p.WS.WriteJSON(msg)
		}
	}
}

func (s *SignalingService) broadcastToRoomRaw(roomID string, msg domain.WsSession) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if shard.conns[roomID] == nil || len(shard.conns[roomID]) == 0 {
		return
	}
	for _, p := range shard.conns[roomID] {
		if p != nil && p.WS != nil {
			_ = p.WS.WriteJSON(msg)
		}
	}
}

func (s *SignalingService) sendToPeerRaw(roomID, peerID string, msg domain.WsSession) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if shard.conns[roomID] == nil {
		return
	}
	if p, exists := shard.conns[roomID][peerID]; exists && p != nil && p.WS != nil {
		_ = p.WS.WriteJSON(msg)
	}
}

func (s *SignalingService) getShardIndex(roomID string) uint32 {
	var hash uint32 = 5381
	for i := 0; i < len(roomID); i++ {
		hash = ((hash << 5) + hash) + uint32(roomID[i])
	}
	return hash % 16
}
