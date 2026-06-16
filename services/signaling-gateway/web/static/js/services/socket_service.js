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
    
    // Подключаемся к L7-балансировщику API Gateway (:8080)
    const socketUrl = `${wsProtocol}${window.location.host}/api/v1/ws?room=${SessionState.roomId}&peer=${SessionState.myPeerId}&mod=${SessionState.isModerator}`;
    
    SessionState.ws = new WebSocket(socketUrl);
    window.ws = SessionState.ws;

    SessionState.ws.onopen = () => {
        // Стартовый пакет регистрации личности в RAM-комнате бэкенда
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
                
                if (Array.isArray(msg.participants)) {
                    for (const p of msg.participants) {
                        SessionState.peerNames[p.id] = p.name;
                        await createPeerConnection(p.id, p.name, false);
                    }
                }
                break;

            // ИСПРАВЛЕНО (Бизнес-функция №1): Нативно принимаем пачку логов из RAM-памяти Go бэкенда при входе
            // FIXED: Captured historical chat array frame payload dump from microservice database container
            case "chat_history_dump":
            case "chat-history-dump":
                if (Array.isArray(msg.logs)) {
                    msg.logs.forEach(l => {
                        logChat(`${l.sender_id || l.sender_name}: ${l.text}`);
                    });
                }
                // Динамически выводим емкость буфера в плашку
                const lbl = document.getElementById('buffer-capacity-lbl');
                if (lbl && msg.logs) lbl.innerText = `[Capacity: ${msg.logs.length}/50k]`;
                break;

            case "peer_joined":
            case "peer-joined":
                const joinedID = msg.peer_id || msg.sender_id;
                const joinedName = msg.sender_name || joinedID;
                SessionState.peerNames[joinedID] = joinedName;
                
                logChat(`[JOIN] Участник ${joinedName} вошел в Mesh-сессию`, "#fbbf24");
                await createPeerConnection(joinedID, joinedName, true);
                break;

            case "peer_left":
            case "peer-left":
                const leftID = msg.peer_id || msg.sender_id;
                logChat(`[LEAVE] Участник покинул созвон`, "#ef4444");
                
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
                // Извлекаем чистый сгенерированный ID файла
                SessionState.currentServerRecordID = msg.file;
                if (window.setServerRecordSessionID) {
                    window.setServerRecordSessionID(msg.file);
                }
                logChat(`[RECORDING] gRPC Стрим-канал к spr-storage открыт. Файл: ${msg.file}.webm`, "#ef4444");
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
