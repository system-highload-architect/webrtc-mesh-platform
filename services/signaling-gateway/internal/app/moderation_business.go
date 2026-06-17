package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"webrtc-mesh-platform/pb/gen" // Сгенерированные protobuf-структуры
	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (s *SignalingService) HandleWsSignal(roomID, peerID string, ws *websocket.Conn, isModerator bool) {
	// 1. ИСПРАВЛЕНО (Identity Plane gRPC): Подключаемся к auth-service на порт :50051
	// FIXED: Bound gRPC client channel context to communicate with identity core auth-service
	authConn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	var authClient gen.AuthenticationBridgeClient
	if err == nil && authConn != nil {
		authClient = gen.NewAuthenticationBridgeClient(authConn)
	}

	// Извлекаем токен авторизации (он у нас прокидывается в peerID или параметрах)
	userRole := "GUEST"
	displayName := peerID

	if authClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		profile, err := authClient.GetSubscriberProfile(ctx, &gen.ProfileRequest{UserId: peerID})
		cancel()
		if err == nil && profile != nil {
			userRole = profile.UserRole
			displayName = profile.Name
		}
	}

	// 2. ИСПРАВЛЕНО (Контур Безопасности Enterprise): Отсекаем гостей от закрытых комнат
	// Если комната содержит маркер "_private", а роль юзера — GUEST, наглухо закрываем туннель!
	if strings.HasSuffix(roomID, "_private") && userRole == "GUEST" {
		_ = ws.WriteJSON(map[string]string{
			"type": "force_kick",
			"text": "🔒 Доступ заблокирован: Эта конференция закрыта для внешних гостей.",
		})
		_ = ws.Close()
		if authConn != nil {
			_ = authConn.Close()
		}
		return
	}

	// Если гость заходит в открытую комнату — нативно маркируем его имя суффиксом
	if userRole == "GUEST" {
		displayName = peerID + "_Guest"
	}

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
				if authConn != nil {
					_ = authConn.Close()
				}
				return
			}
		}
	}
	shard.mu.Unlock()

	grpcConn, err := grpc.Dial("localhost:9082", grpc.WithTransportCredentials(insecure.NewCredentials()))
	var chatClient gen.ChatHistoryBridgeClient
	if err == nil && grpcConn != nil {
		chatClient = gen.NewChatHistoryBridgeClient(grpcConn)
		s.log.Info("✅ [CONTROL PLANE gRPC] Сетевой gRPC-туннель к chat-history-service (:9082) УСПЕШНО ОТКРЫТ!")
	} else {
		s.log.Error("❌ [CONTROL PLANE gRPC] Крах gRPC-туннеля к chat-history-service: %v", err)
	}

	var activeRecordFile *os.File

	defer func() {
		if authConn != nil {
			_ = authConn.Close()
		}
		if grpcConn != nil {
			_ = grpcConn.Close()
		}
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
		s.log.Info("👑 [CONTROL PLANE] Владелец %s в сети! Активация сопряжения комнат [%s]", displayName, roomID)
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

			// ИСПРАВЛЕНО (gRPC Выгрузка Архива чата): Выгребаем логи из выделенного микросервиса по gRPC
			// FIXED: Fetched clean room history dump logs from remote chat-history-service node via gRPC channel
			var historyLogs []map[string]string
			if chatClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				res, err := chatClient.GetRoomChatHistory(ctx, &gen.GetHistoryRequest{RoomId: roomID})
				cancel()
				if err == nil && res != nil {
					for _, logFrame := range res.Logs {
						// ДОБАВЛЕНО: Трейс лог для 100% подтверждения успешной распаковки gRPC кадра!
						s.log.Info("🔥 [gRPC DATA DISPATCH] Успешно извлечена строка лога из базы чата: %s -> %s", logFrame.SenderId, logFrame.Text)
						historyLogs = append(historyLogs, map[string]string{
							"sender_id": logFrame.SenderId,
							"text":      logFrame.Text,
						})
					}
				}
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
			// ИСПРАВЛЕНО (Межсервисный gRPC Аппенд чата): Стримим новые сообщения в чат-сервис на порт :9082
			// FIXED: Dispatched raw chat payload over to dedicated history node for validation and disk commit
			if chatClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				_, _ = chatClient.IngestChatMessage(ctx, &gen.ChatMessagePayload{
					RoomId:      roomID,
					SenderId:    peerID,
					MessageText: incoming.Text,
				})
				cancel()
			}

			s.broadcastToRoomRaw(roomID, domain.WsSession{Type: "chat_broadcast", SenderID: peerID, Text: incoming.Text})
			continue
		}

		if incoming.Type == "control_frame" {
			// ИСПРАВЛЕНО (Централизованный фокус Спикера): Синхронно перестраиваем CSS-сетку у ВСЕХ участников
			// FIXED: Injected master broadcast routines to toggle remote active speaker viewports for all peers
			if incoming.Command == "SET_SPEAKER" && incoming.TargetPeerID != "" {
				s.broadcastMapToRoom(roomID, map[string]string{
					"type":           "focus_speaker",
					"target_peer_id": incoming.TargetPeerID,
				})
				continue
			}
			if incoming.Command == "RESET_SPEAKER" {
				s.broadcastMapToRoom(roomID, map[string]string{
					"type": "reset_speaker",
				})
				continue
			}

			// ИСПРАВЛЕНО (Административный Режим Доклада): Веoverhead-глушение всех микрофонов зала по Gx-аналогу
			// FIXED: Dispatched room-wide force_mute constraint frames to silence all guest track allocations
			if incoming.Command == "GLOBAL_MUTE_AUDIO" {
				s.log.Info("🚨 [CONTROL PLANE] Форсированное глушение аудио-треков зала модератором: %s", peerID)
				s.broadcastMapToRoomExcept(roomID, peerID, map[string]string{
					"type": "force_mute",
				})
				continue
			}

			// ИСПРАВЛЕНО (Административное тушение камер зала): Принудительно отключаем видеопотоки гостей
			if incoming.Command == "GLOBAL_MUTE_VIDEO" {
				s.log.Error("🚨 [CONTROL PLANE] Форсированное отключение видеокамер зала модератором: %s", peerID)
				s.broadcastMapToRoomExcept(roomID, peerID, map[string]string{
					"type": "force_kick", // Фолбэк в перезапуск/кик или force_mute_video в зависимости от UI
				})
				continue
			}

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
