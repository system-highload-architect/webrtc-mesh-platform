import { SessionState } from '../session_context.js';
import { createPeerConnection, removePeerVideo } from './create_peer.js';
import { handleOffer, handleAnswer, handleCandidate } from './webrtc_signaling.js';
import { logChat } from '../chat/render_log.js';
import { toggleUiFreeze } from '../buttons/pause_room.js';

/**
 * initSocketConnection инициализирует туннель сигнализации кластера
 */
export function initSocketConnection() {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
    const socketUrl = `${wsProtocol}${window.location.host}/api/v1/ws?room=${SessionState.roomId}&peer=${SessionState.myPeerId}&mod=${SessionState.isModerator}`;
    
    SessionState.ws = new WebSocket(socketUrl);
    window.ws = SessionState.ws;

    SessionState.ws.onopen = () => {
        SessionState.ws.send(JSON.stringify({
            type: "join",
            room_id: SessionState.roomId,
            sender_name: SessionState.myPeerId
        }));
    };

    SessionState.ws.onmessage = async (event) => {
        const msg = JSON.parse(event.data);
        
        switch (msg.type) {
            case "waiting_for_moderator":
                const workspace = document.getElementById('conference-session-root');
                if (workspace) {
                    workspace.style.pointerEvents = "none";
                    let overlay = document.getElementById('waiting-overlay-node');
                    if (!overlay) {
                        overlay = document.createElement('div');
                        overlay.id = 'waiting-overlay-node';
                        overlay.style.cssText = "position:fixed; top:0; left:0; width:100vw; height:100vh; background:#020617; z-index:99999; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:12px; font-family:monospace;";
                        overlay.innerHTML = `
                            <div style="color:#ecc94b; font-size:2rem; font-weight:bold; animation: pulse 1.5s infinite;">🔒 ROOM LOCK</div>
                            <div style="color:#8b949e; font-size:13px; text-align:center; max-width:400px; line-height:1.5;">${msg.text}</div>
                            <div style="color:#334155; font-size:11px; margin-top:20px;">Платформа Clearway PKI Mesh • Автоматическая активация</div>
                        `;
                        document.body.appendChild(overlay);
                    }
                }
                break;

            case "room_activated":
                if (!SessionState.isModerator) {
                    logChat("[SYSTEM] Владелец вошел! Инициализация автоматического сопряжения комнат...", "#10b981");
                    setTimeout(() => {
                        window.location.reload();
                    }, 400);
                }
                break;

            case "welcome":
                SessionState.myId = msg.sender_id;
                logChat(`[SYSTEM] Зашифрованный Control Plane контур взведен. ID: ${SessionState.myId}`, "#10b981");
                
                // ИСПРАВЛЕНО (Уничтожение гонки офферов): Новый зашедший участник (у которого страница ТОЛЬКО ЧТО загрузилась)
                // сам выступает активным Инициатором коннекта (isInitiator = true) ко всей исторической цепочке старожилов!
                // FIXED: Enforced incoming peer initialization schema (isInitiator = true) inside welcome handler frame
                if (Array.isArray(msg.participants)) {
                    for (const p of msg.participants) {
                        SessionState.peerNames[p.id] = p.name;
                        await createPeerConnection(p.id, p.name, true);
                    }
                }
                break;

            case "chat_history_dump":
            case "chat-history-dump":
                if (Array.isArray(msg.logs)) {
                    msg.logs.forEach(l => {
                        logChat(`${l.sender_id || l.sender_name}: ${l.text}`);
                    });
                }
                const chatBoxNode = document.getElementById('chat-box');
                if (chatBoxNode) chatBoxNode.scrollTop = chatBoxNode.scrollHeight;
                break;

            case "peer_joined":
            case "peer-joined":
                const joinedID = msg.peer_id || msg.sender_id;
                const joinedName = msg.sender_name || joinedID;
                SessionState.peerNames[joinedID] = joinedName;
                
                logChat(`[JOIN] Участник ${joinedName} вошел в Mesh-сессию`, "#fbbf24");
                
                // ИСПРАВЛЕНО (Пассивное ожидание старожилов): Старожилы комнаты (включая Давида), чьи страницы уже давно открыты,
                // переводят канал в состояние пассивного ожидания (isInitiator = false) и чисто принимают входящий оффер от гостя!
                await createPeerConnection(joinedID, joinedName, false);
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
                SessionState.currentServerRecordID = msg.file;
                if (window.setServerRecordSessionID) { window.setServerRecordSessionID(msg.file); }
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
                if (SessionState.localStream) { SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false); }
                const audioBtn = document.getElementById('audio-toggle');
                if (audioBtn) { audioBtn.innerText = "🎤 Микрофон: выкл"; audioBtn.style.backgroundColor = "#7f1d1d"; }
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
