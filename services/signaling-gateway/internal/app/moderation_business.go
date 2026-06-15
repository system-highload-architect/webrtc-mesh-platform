package app

import (
	"time"

	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// HandleWsSignal терминирует сигнальный поток и жестко валидирует права по JWT токену личности (Req. 1, 2 & 5)
func (s *SignalingService) HandleWsSignal(roomID, tokenStr string, ws *websocket.Conn) {
	// 1. ПАТТЕРН БЕЗОПАСНОСТИ (Req. 5): На лету вскрываем криптографическую подпись JWT
	claims, err := s.ValidateAndParseJwt(tokenStr)
	if err != nil {
		s.log.Error("[SECURITY ALERT] Отклонена попытка входа по поддельному JWT: %v", err)
		_ = ws.WriteJSON(map[string]any{"type": "auth_error", "reason": "invalid_jwt_token"})
		_ = ws.Close()
		return
	}

	// Извлекаем реальную идентичность человека и роль из защищенного токена
	peerID := claims.UserID
	isModerator := claims.Role == "ORGANIZER"

	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	// 2. Атомарно регистрируем сессию абонента в RAM-шарде за O(1)
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

	s.log.Info("PEER VERIFIED -> Участник [%s] с ролью [%s] вошел в комнату [%s]", peerID, claims.Role, roomID)

	// 3. Graceful-зачистка сетевых сокетов при разрыве связи
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

	// Оповещаем комнату о входе нового верифицированного участника
	s.broadcastToRoom(roomID, map[string]any{
		"type":    "peer_joined",
		"peer_id": peerID,
	})

	// 4. Бесконечный цикл чтения WebSocket-фреймов
	for {
		var msg map[string]any
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {

		case "sdp_offer", "sdp_answer", "ice_candidate":
			target, _ := msg["target_peer_id"].(string)
			s.sendToPeer(roomID, target, msg)

		case "control_frame":
			// Жесткая серверная проверка b2b прав модератора из RAM структуры, привязанной к JWT
			if !isModerator {
				s.log.Error("[SECURITY VIOLATION] Попытка несанкционированной модерации от Peer: %s", peerID)
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
			s.broadcastToRoom(roomID, map[string]any{
				"type":      "chat_broadcast",
				"sender_id": peerID,
				"text":      msg["text"],
			})
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

func (s *SignalingService) sendToPeer(roomID, peerID string, msg any) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if p, exists := shard.conns[roomID][peerID]; exists {
		_ = p.WS.WriteJSON(msg)
	}
}
