import { SessionState } from '../session_context.js';
import { createPeerConnection, removePeerVideo } from './create_peer.js';
import { handleOffer, handleAnswer, handleCandidate } from './webrtc_signaling.js';
import { logChat } from '../chat/render_log.js';
import { toggleUiFreeze } from '../buttons/pause_room.js';

/**
 * initSocketConnection инициализирует защищенный полнодуплексный туннель сигнализации кластера
 */
export function initSocketConnection() {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
    
    // Подключаемся к L7-балансировщику API Gateway (:8080), который проксирует нас на шлюз (:8081)
    const socketUrl = `${wsProtocol}${window.location.host}/api/v1/ws?room=${SessionState.roomId}&peer=${SessionState.myPeerId}&mod=${SessionState.isModerator}`;
    
    SessionState.ws = new WebSocket(socketUrl);
    window.ws = SessionState.ws; // Сохраняем глобальный алиас для обратной совместимости

    SessionState.ws.onopen = () => {
        // Выстреливаем стартовый b2b-пакет регистрации личности в RAM-комнате бэкенда
        SessionState.ws.send(JSON.stringify({
            type: "join",
            room_id: SessionState.roomId,
            sender_name: SessionState.myPeerId
        }));
    };

    SessionState.ws.onmessage = async (event) => {
        const msg = JSON.parse(event.data);
        
        switch (msg.type) {
            case "welcome":
                SessionState.myId = msg.sender_id;
                logChat(`[SYSTEM] Зашифрованный Control Plane контур взведен. ID: ${SessionState.myId}`, "#10b981");
                
                // Наполняем in-memory мапу участников, которые зашли в RAM-комнату РАНЬШЕ нас
                if (Array.isArray(msg.participants)) {
                    msg.participants.forEach(p => {
                        SessionState.peerNames[p.id] = p.name;
                    });
                }
                break;

            case "peer_joined":
            case "peer-joined":
                const joinedID = msg.peer_id || msg.sender_id;
                const joinedName = msg.sender_name || joinedID;
                SessionState.peerNames[joinedID] = joinedName;
                
                logChat(`[JOIN] Участник ${joinedName} вошел в Mesh-сессию`, "#fbbf24");
                
                // Триггерим нативную генерацию P2P-плеча WebRTC
                await createPeerConnection(joinedID, joinedName, true);
                break;

            case "peer_left":
            case "peer-left":
                const leftID = msg.peer_id || msg.sender_id;
                logChat(`[LEAVE] Участник покинул созвон`, "#ef4444");
                
                // Утилизируем видео-ноду из DOM-дерева и закрываем дескриптор RTCPeerConnection
                removePeerVideo(leftID);
                delete SessionState.peerNames[leftID];
                break;

            case "chat_broadcast":
            case "chat":
                logChat(`${msg.sender_id || msg.sender_name}: ${msg.text}`);
                break;

            case "offer":
                SessionState.peerNames[msg.sender_id] = msg.sender_name;
                await handleOffer(msg);
                break;

            case "answer":
                await handleAnswer(msg);
                break;

            case "candidate":
                await handleCandidate(msg);
                break;

            case "record_started":
                const parts = msg.file.split(/[\\/]/);
                SessionState.currentServerRecordID = parts[parts.length - 1].replace(".webm", "");
                logChat(`[RECORDING] NVMe файл записи сессии открыт: ${SessionState.currentServerRecordID}`, "#ef4444");
                break;

            case "room_paused":
                toggleUiFreeze(true);
                break;

            case "room_resumed":
                toggleUiFreeze(false);
                break;
                
            case "force_mute":
                logChat("[ORCHESTRATION] Администратор ограничил ваш микрофон (Режим доклада).", "#ef4444");
                SessionState.isAudioMuted = true;
                if (SessionState.localStream) {
                    SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false);
                }
                const audioBtn = document.getElementById('audio-toggle');
                if (audioBtn) {
                    audioBtn.innerText = "🎤 Микрофон: выкл";
                    audioBtn.style.backgroundColor = "#7f1d1d";
                }
                break;

            case "force_kick":
                alert("Вы были удалены из конференции модератором сессии.");
                window.location.href = "/";
                break;
        }
    };

    SessionState.ws.onerror = (err) => {
        console.error("[SOCKET ERROR] Сбой сигнального туннеля Gateway:", err);
    };
}
