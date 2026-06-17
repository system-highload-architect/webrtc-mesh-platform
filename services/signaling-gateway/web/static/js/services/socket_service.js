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
        const roomDisplay = document.getElementById('room-lbl');
        if (roomDisplay) roomDisplay.innerText = SessionState.roomId;

        const roleBadge = document.getElementById('role-badge');
        if (roleBadge) {
            roleBadge.innerText = SessionState.isModerator ? "👑 ОРГАНИЗАТОР КОНТУРА" : "📡 СОТРУДНИК СЕССИИ";
            roleBadge.style.color = SessionState.isModerator ? "#ecc94b" : "#3b82f6";
        }

        SessionState.ws.send(JSON.stringify({
            type: "join", room_id: SessionState.roomId, sender_name: SessionState.myPeerId
        }));
    };

    SessionState.ws.onmessage = async (event) => {
        const msg = JSON.parse(event.data);
        const grid = document.getElementById('video-grid');
        
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
                    logChat("[SYSTEM] Владелец вошел! Инициализация automatic сопряжения комнат...", "#10b981");
                    setTimeout(() => {
                        window.location.reload();
                    }, 400);
                }
                break;

            case "welcome":
                SessionState.myId = msg.sender_id;
                logChat(`[SYSTEM] Зашифрованный Control Plane контур взведен. ID: ${SessionState.myId}`, "#10b981");
                
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

            case "peer-joined":
                const joinedID = msg.peer_id || msg.sender_id;
                const joinedName = msg.sender_name || joinedID;
                SessionState.peerNames[joinedID] = joinedName;
                
                logChat(`[JOIN] Участник ${joinedName} вошел в Mesh-сессию`, "#fbbf24");
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
                
            // ИСПРАВЛЕНО (Безвозвратный лок микрофонов зала ТЗ Давида):
            // Насильно гасим звук рядового сотрудника и намертво блокируем кнопку включения в футере!
            // FIXED: Activated guest audio track lock context and disabled footer button node clicks
            case "force_mute_audio_lock":
                logChat("[ORCHESTRATION] Ведущий заблокировал ваш микрофон (Режим доклада). Включение запрещено.", "#ef4444");
                SessionState.isAudioMuted = true;
                
                if (SessionState.localStream) {
                    SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false);
                }
                
                const audioBtn = document.getElementById('audio-toggle');
                if (audioBtn) {
                    audioBtn.innerText = "🎤 Микрофон заблокирован";
                    audioBtn.style.backgroundColor = "#451a03"; // Предупреждающий b2b-цвет блокировки устройства
                    audioBtn.style.borderColor = "#7c2d12";
                    audioBtn.style.pointerEvents = "none"; // Глухой аппаратный запрет клика мыши!
                    audioBtn.style.opacity = "0.5";
                }
                break;

            // ИСПРАВЛЕНО (Безвозвратный лок видеокамер зала ТЗ Давида):
            // Насильно гасим камеру рядового сотрудника и намертво блокируем кнопку включения в футере!
            // FIXED: Activated guest video track lock context and disabled footer button node clicks
            case "force_mute_video_lock":
                logChat("[ORCHESTRATION] Ведущий заблокировал вашу видеокамеру (Режим доклада). Включение запрещено.", "#ef4444");
                SessionState.isVideoMuted = true;
                
                if (SessionState.localStream) {
                    SessionState.localStream.getVideoTracks().forEach(t => t.enabled = false);
                }
                
                const videoBtn = document.getElementById('video-toggle');
                if (videoBtn) {
                    videoBtn.innerText = "📷 Камера заблокирована";
                    videoBtn.style.backgroundColor = "#451a03";
                    videoBtn.style.borderColor = "#7c2d12";
                    videoBtn.style.pointerEvents = "none"; // Глухой аппаратный запрет клика мыши!
                    videoBtn.style.opacity = "0.5";
                }
                const localVideo = document.getElementById('local-video');
                if (localVideo) localVideo.style.opacity = "0.15"; // Уводим локальное превью в прозрачность
                break;
                
            case "force_kick":
                alert("Вы были удалены из конференции модератором сессии.");
                window.location.href = "/";
                break;
        }
    };

    // Делегированный перехват кликов по кнопке "⭐ Спикер" на видеоплитках гостей
    document.addEventListener('click', (e) => {
        if (e.target && e.target.classList.contains('target-speaker-btn')) {
            let targetPeerID = e.target.getAttribute('data-peer');
            if (!targetPeerID) return;

            if (targetPeerID === "LOCAL_ME") { targetPeerID = SessionState.myPeerId; }

            const localContainer = document.getElementById('local-video-container');
            const targetWrapper = targetPeerID === SessionState.myPeerId ? localContainer : document.getElementById(`video-${targetPeerID}`);
            const isAlreadySpeaker = targetWrapper ? targetWrapper.classList.contains('active-speaker-focus') : false;

            if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                SessionState.ws.send(JSON.stringify({
                    type: "control_frame", command: isAlreadySpeaker ? "RESET_SPEAKER" : "SET_SPEAKER", target_peer_id: targetPeerID
                }));
            }
        }
    });

    // Клики кнопок админ панели
    setTimeout(() => {
        // ИСПРАВЛЕНО (Авторитарный блок звука ТЗ Давида): Пушим GLOBAL_MUTE_AUDIO и защищаем ID Спикера
        // FIXED: Dispatched control frame payload to block room tracks while filtering active speaker ID
        const muteAllAudioBtn = document.getElementById('mute-all-audio-btn');
        if (muteAllAudioBtn) {
            muteAllAudioBtn.onclick = () => {
                if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                    const currentActiveSpeaker = SessionState.activeSpeakerId || "";
                    SessionState.ws.send(JSON.stringify({
                        type: "control_frame",
                        command: "GLOBAL_MUTE_AUDIO",
                        target_peer_id: currentActiveSpeaker
                    }));
                }
            };
        }

        // ИСПРАВЛЕНО (Авторитарный блок видео ТЗ Давида): Пушим GLOBAL_MUTE_VIDEO и защищаем ID Спикера
        // FIXED: Dispatched control frame payload to block room tracks while filtering active speaker ID
        const muteAllVideoBtn = document.getElementById('mute-all-video-btn');
        if (muteAllVideoBtn) {
            muteAllVideoBtn.onclick = () => {
                if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                    const currentActiveSpeaker = SessionState.activeSpeakerId || "";
                    SessionState.ws.send(JSON.stringify({
                        type: "control_frame",
                        command: "GLOBAL_MUTE_VIDEO",
                        target_peer_id: currentActiveSpeaker
                    }));
                }
            };
        }
    }, 1000);

    SessionState.ws.onerror = (err) => { console.error("[SOCKET ERROR]:", err); };
}
