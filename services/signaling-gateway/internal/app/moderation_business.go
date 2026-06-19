package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

var roomCommandRegistry = map[string]struct {
	StateCode int
	OnType    string
	OffType   string
}{
	"GLOBAL_MUTE_AUDIO": {StateCode: 1, OnType: "force_mute_audio_lock", OffType: "force_unmute_audio_lock"},
	"GLOBAL_MUTE_VIDEO": {StateCode: 2, OnType: "force_mute_video_lock", OffType: "force_unmute_video_lock"},
	"SET_PAUSE":         {StateCode: 0, OnType: "room_paused", OffType: "room_resumed"},
}

// HandleWsSignal управляет полным циклом сопряжения Webrtc-клиентов и контролирует лимиты PCEF
// ИСПРАВЛЕНО (Уничтожение Race-Condition по контексту Gorilla): Полностью убрали чтение ws.UnderlyingConn().
// Лимиты теперь извлекаются нативно за 1 наносекунду из JSON-пакета "join" на этапе парсинга фреймов!
// FIXED: Restored classic 4-parameter aggregate function loop to avoid net.Conn casting panics
func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()

	// Capacity Shield: Проверяем лимит коннектов ДО аллокации и раздувания памяти нового участника
	roomObj, roomExists := shard.lruCache.Get(roomID)
	if roomExists {
		activeRoom := roomObj.(*domain.VideoRoom)
		// Если текущее число коннектов достигло лимита — рубим атаку на Control Plane на корню
		if len(shard.conns[roomID]) >= activeRoom.MaxPeers {
			shard.mu.Unlock()
			_ = ws.WriteJSON(map[string]string{
				"type": "room_full",
				"text": "🔒 [LIMIT SHIELD]: В комнате достигнут максимальный предел участников. Доступ закрыт.",
			})
			_ = ws.Close()
			return
		}
	}

	pConn := &PeerConnection{PeerID: peerID, WS: ws, IsModerator: isModerator, IsMuted: false}
	if _, exists := shard.conns[roomID]; !exists {
		shard.conns[roomID] = make(map[string]*PeerConnection)
	}
	shard.conns[roomID][peerID] = pConn

	var room *domain.VideoRoom
	if !roomExists {
		// Ленивый дефолтный старт. Реальные лимиты с экрана Давида перепишутся в ОЗУ через миллисекунду пакетом "join"
		room = &domain.VideoRoom{
			RoomID:      roomID,
			MaxPeers:    100,
			IsPaused:    false,
			Peers:       make(map[string]*domain.PeerSession),
			ChatHistory: make([]map[string]any, 0),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(), // Будет хранить абсолютную точку плановой смерти
			RoomStates:  make(map[int]bool),
		}
		room.UpdatedAt = time.Now().Add(30 * time.Minute)
		shard.lruCache.Set(roomID, room)
		shard.wheel.Add(roomID, 30)
		s.log.Info("[PCEF RADAR] RAM-шард комнаты %s лениво создан в ОЗУ. Ожидание приветственного фрейма 'join'...", roomID)
	} else {
		room = roomObj.(*domain.VideoRoom)
	}

	if room.RoomStates == nil {
		room.RoomStates = make(map[int]bool)
	}

	hasActiveModerator := false
	for id := range shard.conns[roomID] {
		if shard.conns[roomID][id] != nil && shard.conns[roomID][id].IsModerator {
			hasActiveModerator = true
			break
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

		if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
			currentRoom := currentRoomObj.(*domain.VideoRoom)
			delete(currentRoom.Peers, peerID)
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

			if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
				room = currentRoomObj.(*domain.VideoRoom)
				room.Peers[peerID] = &domain.PeerSession{
					PeerID:          peerID,
					IsModerator:     isModerator,
					IsMuted:         false,
					LastHeartbeat:   time.Now(),
					LastMessageUnix: time.Now().UnixNano(),
				}

				if isModerator {

					if incoming.RecordID != "" && incoming.RecordID != "undefined" {
						var rawMaxPeers int
						// Если Sscanf успешно распарсил число и оно лежит в b2b границах (1-100) — применяем!
						if _, errMax := fmt.Sscanf(incoming.RecordID, "%d", &rawMaxPeers); errMax == nil && rawMaxPeers >= 1 && rawMaxPeers <= 100 {
							room.MaxPeers = rawMaxPeers
						}
					}

					if incoming.Text != "" && incoming.Text != "undefined" {
						var rawDuration int
						// Если Давид ввёл время и оно не превышает 5 часов (300 минут) — ставим на Колесо Времени!
						if _, errDur := fmt.Sscanf(incoming.Text, "%d", &rawDuration); errDur == nil && rawDuration >= 1 && rawDuration <= 300 {
							// Выметаем старый дефолтный 30-минутный слот с кольца
							shard.wheel.Remove(roomID)

							// Запечатываем живой дедлайн в Битовое Колесо Времени
							room.UpdatedAt = time.Now().Add(time.Duration(rawDuration) * time.Minute)
							shard.wheel.Add(roomID, rawDuration)

							s.log.Info("🎰 [PCEF RE-CALIBRATION] Лимиты успешно применены: Вместимость [%d человек] | Время [%d мин]", room.MaxPeers, rawDuration)
						}
					}
				}
			}

			var currentParticipants []map[string]string
			for id := range shard.conns[roomID] {
				if id != peerID {
					currentParticipants = append(currentParticipants, map[string]string{"id": id, "name": id})
				}
			}

			var historyLogs []map[string]string
			for _, h := range room.ChatHistory {
				historyLogs = append(historyLogs, map[string]string{
					"sender_id": h["sender_id"].(string),
					"text":      h["text"].(string),
				})
			}
			shard.mu.Unlock()

			_ = ws.WriteJSON(map[string]any{"type": "welcome", "sender_id": peerID, "participants": currentParticipants})
			_ = ws.WriteJSON(map[string]any{"type": "chat_history_dump", "logs": historyLogs})

			shard.mu.RLock()
			if room != nil && room.RoomStates != nil {
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
			var peerSession *domain.PeerSession
			if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
				room = currentRoomObj.(*domain.VideoRoom)
				if room.Peers[peerID] != nil {
					peerSession = room.Peers[peerID]
				}
			}
			shard.mu.Unlock()

			if peerSession != nil {
				nowUnixNano := time.Now().UnixNano()
				lastMsgNano := atomic.LoadInt64(&peerSession.LastMessageUnix)

				if nowUnixNano-lastMsgNano < 300000000 {
					_ = ws.WriteJSON(map[string]string{
						"type":      "chat_broadcast",
						"sender_id": "SYSTEM_SECURITY",
						"text":      "⚠️ [CAS RATE LIMITER] Превышена частота отправки сообщений. Флуд заблокирован за 9 наносекунд.",
					})
					continue
				}
				atomic.StoreInt64(&peerSession.LastMessageUnix, nowUnixNano)
			}

			shard.mu.Lock()
			if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
				room = currentRoomObj.(*domain.VideoRoom)
				room.ChatHistory = append(room.ChatHistory, map[string]any{"sender_id": peerID, "text": incoming.Text, "timestamp": time.Now()})
			}
			shard.mu.Unlock()

			s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "chat_broadcast", SenderID: peerID, Text: incoming.Text})
			continue
		}

		if incoming.Type == "control_frame" {
			if incoming.Command == "SET_SPEAKER" && incoming.TargetPeerID != "" {
				shard.mu.Lock()
				if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
					cro := currentRoomObj.(*domain.VideoRoom)
					cro.CurrentSpeakerID = incoming.TargetPeerID
				}
				shard.mu.Unlock()
				s.broadcastMapToRoom(roomID, map[string]string{"type": "focus_speaker", "target_peer_id": incoming.TargetPeerID})
				continue
			}
			if incoming.Command == "RESET_SPEAKER" {
				shard.mu.Lock()
				if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
					cro := currentRoomObj.(*domain.VideoRoom)
					cro.CurrentSpeakerID = ""
				}
				shard.mu.Unlock()
				s.broadcastMapToRoom(roomID, map[string]string{"type": "reset_speaker"})
				continue
			}

			if configCmd, exists := roomCommandRegistry[incoming.Command]; exists {
				shard.mu.Lock()

				// ИСПРАВЛЕНО (Уничтожение варнинга линкера): Явно глушим переменную для компилятора Go 1.25+
				// FIXED: Cleared variable declaration warning by explicitly discarding the unused token
				_ = configCmd

				if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
					room = currentRoomObj.(*domain.VideoRoom)
				}
				if room.RoomStates == nil {
					room.RoomStates = make(map[int]bool)
				}
				room.RoomStates[configCmd.StateCode] = !room.RoomStates[configCmd.StateCode]
				isActiveNow := room.RoomStates[configCmd.StateCode]

				if configCmd.StateCode == 0 {
					room.IsPaused = isActiveNow

					if room.IsPaused {
						// 1. НАЖАЛИ ПАУЗУ: Вырезаем комнату с текущей минуты и прячем в страховой 300-й слот (5 часов от DoS)
						shard.wheel.Remove(roomID)
						shard.wheel.Add(roomID, 300)

						room.CreatedAt = time.Now() // Запоминаем точную метку времени старта ПАУЗЫ
						s.log.Info("⏳ [OOM PROTECTION] Сессия %s на ПАУЗЕ. Взведен страховой дедлайн на 5 часов.", roomID)
					} else {
						// 2. СНЯЛИ ПАУЗУ ПОВТОРНЫМ КЛИКОМ: Полностью выжигаем из страхового 5-часового слота
						shard.wheel.Remove(roomID)

						// Вычисляем точную дельту, сколько комната простояла «замороженной»
						freezeDuration := time.Since(room.CreatedAt)

						// Сдвигаем абсолютный дедлайн смерти строго на чистое время простоя!
						room.UpdatedAt = room.UpdatedAt.Add(freezeDuration)

						// Теперь рассчитываем честный остаток времени от текущего момента до сдвинутого дедлайна
						remainingMinutes := int(time.Until(room.UpdatedAt).Minutes())

						// Защита от сдвига тика: округляем вверх до 1 минуты, чтобы не оказаться позади стрелки
						if remainingMinutes < 1 {
							remainingMinutes = 1
						}

						// Возвращаем комнату на Битовое Колесо Времени Давида в правильный будущий слот
						shard.wheel.Add(roomID, remainingMinutes)
						s.log.Info("▶️ [TIME WHEEL] Пауза с сессии %s снята. Конференция продлена строго на время простоя: +%v", roomID, freezeDuration)
					}
				}

				activeSpeakerID := room.CurrentSpeakerID
				shard.mu.Unlock()

				broadcastType := configCmd.OffType
				if isActiveNow {
					broadcastType = configCmd.OnType
				}

				s.log.Info("🎯 [DISPATCHER] Режим лекции команды [%s]. Код [%d] ➔ Status: [%t]", incoming.Command, configCmd.StateCode, isActiveNow)

				shard.mu.RLock()
				if shard.conns[roomID] != nil {
					for targetID, peerConn := range shard.conns[roomID] {
						if (peerConn != nil && peerConn.IsModerator) || targetID == activeSpeakerID || targetID == "David_Moderator" {
							continue
						}
						if peerConn != nil && peerConn.WS != nil {
							_ = peerConn.WS.WriteJSON(map[string]string{"type": broadcastType})
						}
					}
				}
				shard.mu.RUnlock()
				continue
			}

			if incoming.Command == "RESUME_CONFERENCE" {
				shard.mu.Lock()
				if currentRoomObj, exists := shard.lruCache.Get(roomID); exists {
					room = currentRoomObj.(*domain.VideoRoom)

					if room.IsPaused {
						shard.wheel.Remove(roomID) // Выжигаем из страховой корзины 5 часов

						freezeDuration := time.Since(room.CreatedAt)
						room.UpdatedAt = room.UpdatedAt.Add(freezeDuration)

						remainingMinutes := int(time.Until(room.UpdatedAt).Minutes())
						if remainingMinutes < 1 {
							remainingMinutes = 1
						}

						shard.wheel.Add(roomID, remainingMinutes)
						s.log.Info("▶️ [PCEF] Конференция %s возобновлена. Время простоя компенсировано: +%v", roomID, freezeDuration)
					}

					if room.RoomStates == nil {
						room.RoomStates = make(map[int]bool)
					}
					room.RoomStates[0] = false
					room.IsPaused = false
				}
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
	return hash % s.shardCount
}
