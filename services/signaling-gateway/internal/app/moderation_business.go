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

func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()

	// ИСПРАВЛЕНО (Pre-Registration Routing ТЗ): Регистрируем сетевой сокет в мапу соединений в ПЕРВУЮ ОЧЕРЕДЬ!
	// Теперь широковещательный метод вещания гарантированно увидит дескрипторы сокетов ждущих сотрудников!
	// FIXED: Mounted socket descriptor to global room connections array early to ensure broadcast visibility
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

	// Обсчитываем наличие модератора
	roomExists := false
	hasActiveModerator := false
	if _, exists := shard.rooms[roomID]; exists {
		roomExists = true
		for id := range shard.conns[roomID] {
			if shard.conns[roomID][id] != nil && shard.conns[roomID][id].IsModerator {
				hasActiveModerator = true
				break
			}
		}
	}

	// Если заходит рядовой Сотрудник, а Владельца в комнате ЕЕТ — шлем пакет удержания и паркуем рутину
	if !isModerator && (!roomExists || !hasActiveModerator) {
		shard.mu.Unlock()
		_ = ws.WriteJSON(map[string]string{
			"type": "waiting_for_moderator",
			"text": "Конференция еще не началась. Ожидайте авторизации Владельца комнаты...",
		})

		// Асинхронно паркуем рутину сотрудника на чтение, освобождая мьютекс
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
			room := shard.rooms[roomID]
			room.ChatHistory = append(room.ChatHistory, map[string]any{"sender_id": peerID, "text": incoming.Text, "timestamp": time.Now()})
			shard.mu.Unlock()

			s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "chat_broadcast", SenderID: peerID, Text: incoming.Text})
			continue
		}

		// Находится внутри HandleWsSignal -> цикл чтения сообщений в moderation_business.go:
		if incoming.Type == "control_frame" {
			// ИСПРАВЛЕНО (Глобальный Спикер на бэкенде): Ретранслируем управляющие фреймы модератора на весь зал!
			// FIXED: Injected master broadcast routines to toggle remote active speaker viewports for all peers
			if incoming.Command == "SET_SPEAKER" && incoming.TargetPeerID != "" {
				s.log.Info("👑 [gRPC ORCHESTRATION] Назначен глобальный спикер: %s в комнате %s", incoming.TargetPeerID, roomID)
				s.broadcastMapToRoom(roomID, map[string]string{
					"type":           "focus_speaker",
					"target_peer_id": incoming.TargetPeerID,
				})
				continue
			}
			if incoming.Command == "RESET_SPEAKER" {
				s.log.Info("👑 [gRPC ORCHESTRATION] Сброс глобального спикера в комнате %s", roomID)
				s.broadcastMapToRoom(roomID, map[string]string{
					"type": "reset_speaker",
				})
				continue
			}

			// Дальше идет твой стандартный, неизмененный блок команд (SET_PAUSE, MUTE_AUDIO и т.д.)
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
				s.log.Info("[REC ENGINE] Серверный файл записи запечатан.")
				if activeRecordFile != nil {
					_ = activeRecordFile.Close()
					activeRecordFile = nil
				}
				continue
			}

			if incoming.Command == "GLOBAL_MUTE_AUDIO" {
				s.log.Info("🚨 [CONTROL PLANE] Запуск режима лекции: Блокировка звука пассивного зала")

				idx := s.getShardIndex(roomID)
				shard := s.shards[idx]
				shard.mu.RLock()

				if shard.conns[roomID] != nil {
					for targetID, peerConn := range shard.conns[roomID] {
						// Если пир — это Создатель (Давид) или назначенный Спикер (TargetPeerID с фронта) — пропускаем!
						if targetID == "David_Moderator" || targetID == incoming.TargetPeerID {
							continue
						}
						if peerConn != nil && peerConn.WS != nil {
							_ = peerConn.WS.WriteJSON(map[string]string{
								"type": "force_mute_audio_lock",
							})
						}
					}
				}
				shard.mu.RUnlock()
				continue
			}

			if incoming.Command == "GLOBAL_MUTE_VIDEO" {
				s.log.Info("🚨 [CONTROL PLANE] Запуск режима лекции: Блокировка видеокамер пассивного зала")

				idx := s.getShardIndex(roomID)
				shard := s.shards[idx]
				shard.mu.RLock()

				if shard.conns[roomID] != nil {
					for targetID, peerConn := range shard.conns[roomID] {
						if targetID == "David_Moderator" || targetID == incoming.TargetPeerID {
							continue
						}
						if peerConn != nil && peerConn.WS != nil {
							_ = peerConn.WS.WriteJSON(map[string]string{
								"type": "force_mute_video_lock",
							})
						}
					}
				}
				shard.mu.RUnlock()
				continue
			}
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
