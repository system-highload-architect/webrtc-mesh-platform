import { SessionState } from '../session_context.js';
import { createPeerConnection, removePeerVideo } from './create_peer.js';
import { handleOffer, handleAnswer, handleCandidate } from './webrtc_signaling.js';
import { logChat } from '../chat/render_log.js';
import { toggleUiFreeze } from '../buttons/pause_room.js';
import { initVectorDrawingEngine, handleRemoteVectorInjected } from './vector_draw.js';

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

        initVectorDrawingEngine();

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
                if (SessionState.isModerator) break; // Защита Ведущего
                logChat("[ORCHESTRATION] Ведущий ограничил ваш микрофон (Режим доклада). Включение запрещено.", "#ef4444");
                SessionState.isAudioMuted = true;
                if (SessionState.localStream) SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false);
                const audioBlockBtn = document.getElementById('audio-toggle');
                if (audioBlockBtn) {
                    audioBlockBtn.innerText = "🎤 Микрофон заблокирован";
                    audioBlockBtn.style.backgroundColor = "#451a03"; // Бордовый b2b-цвет блокировки
                    audioBlockBtn.style.borderColor = "#7c2d12";
                    audioBlockBtn.style.pointerEvents = "none";      // Глухой аппаратный запрет клика
                    audioBlockBtn.style.opacity = "0.5";
                }
                break;

            case "force_unmute_audio_lock":
                if (SessionState.isModerator) break;
                logChat("[ORCHESTRATION] Ведущий снял блокировку звука. Вы можете включить микрофон.", "#10b981");
                SessionState.isAudioMuted = false;
                if (SessionState.localStream) SessionState.localStream.getAudioTracks().forEach(t => t.enabled = true);
                const audioUnblockBtn = document.getElementById('audio-toggle');
                if (audioUnblockBtn) {
                    audioUnblockBtn.innerText = "🎤 Микрофон: вкл";
                    audioUnblockBtn.style.backgroundColor = "#1e293b"; // Возвращаем дефолтный b2b-синий цвет
                    audioUnblockBtn.style.borderColor = "#334155";
                    audioUnblockBtn.style.pointerEvents = "auto";      // ВОЗВРАЩАЕМ КЛИКАБЕЛЬНОСТЬ МЫШИ!
                    audioUnblockBtn.style.opacity = "1";
                }
                break;

            case "force_mute_video":
                logChat("[ORCHESTRATION] Модератор точечно отключил вашу видеокамеру.", "#ef4444");
                SessionState.isVideoMuted = true;
                if (SessionState.localStream) SessionState.localStream.getVideoTracks().forEach(t => t.enabled = false);
                const forcedCamBtn = document.getElementById('video-toggle');
                if (forcedCamBtn) { forcedCamBtn.innerText = "📷 Камера: выкл"; forcedCamBtn.style.backgroundColor = "#7f1d1d"; }
                const forcedLocalVideoElement = document.getElementById('local-video');
                if (forcedLocalVideoElement) forcedLocalVideoElement.style.opacity = "0.15";
                break;
                
            case "force_mute_video_lock":
                if (SessionState.isModerator) break; // Защита Ведущего
                logChat("[ORCHESTRATION] Ведущий ограничил вашу видеокамеру (Режим доклада). Включение запрещено.", "#ef4444");
                SessionState.isVideoMuted = true;
                if (SessionState.localStream) SessionState.localStream.getVideoTracks().forEach(t => t.enabled = false);
                const videoBlockBtn = document.getElementById('video-toggle');
                if (videoBlockBtn) {
                    videoBlockBtn.innerText = "📷 Камера заблокирована";
                    videoBlockBtn.style.backgroundColor = "#451a03";
                    videoBlockBtn.style.borderColor = "#7c2d12";
                    videoBlockBtn.style.pointerEvents = "none";      // Глухой аппаратный запрет клика
                    videoBlockBtn.style.opacity = "0.5";
                }
                const localVideoElement = document.getElementById('local-video');
                if (localVideoElement) localVideoElement.style.opacity = "0.15";
                break;

            case "force_unmute_video_lock":
                if (SessionState.isModerator) break;
                logChat("[ORCHESTRATION] Ведущий снял блокировку видеокамер. Вы можете включить видео.", "#10b981");
                SessionState.isVideoMuted = false;
                if (SessionState.localStream) SessionState.localStream.getVideoTracks().forEach(t => t.enabled = true);
                const videoUnblockBtn = document.getElementById('video-toggle');
                if (videoUnblockBtn) {
                    videoUnblockBtn.innerText = "📷 Камера: вкл";
                    videoUnblockBtn.style.backgroundColor = "#1e293b";
                    videoUnblockBtn.style.borderColor = "#334155";
                    videoUnblockBtn.style.pointerEvents = "auto";      // ВОЗВРАЩАЕМ КЛИКАБЕЛЬНОСТЬ МЫШИ!
                    videoUnblockBtn.style.opacity = "1";
                }
                const localVideoRestore = document.getElementById('local-video');
                if (localVideoRestore) localVideoRestore.style.opacity = "1";
                break;

        case "focus_speaker":
                SessionState.activeSpeakerId = msg.target_peer_id;
                logChat(`// [ORCHESTRATION] Внимание зала зафиксировано на спикере: ${msg.target_peer_id}. Ему выдан иммунитет от мьюта.`, "#ecc94b");
                
                // Находим текстовые метки имен на плитках и вешаем префикс
                document.querySelectorAll('.peer-name, #local-name-label').forEach(el => {
                    el.innerText = el.innerText.replace(" [🎙️ ДОКЛАДЧИК]", ""); // Очищаем старые метки
                });
                
                const isMeSpeaker = msg.target_peer_id === SessionState.myPeerId || msg.target_peer_id === "David_Moderator";
                const targetLabel = isMeSpeaker ? 
                    document.getElementById('local-name-label') : 
                    document.querySelector(`#video-${msg.target_peer_id} .peer-name`);
                
                if (targetLabel) {
                    targetLabel.innerText += " [🎙️ ДОКЛАДЧИК]";
                    targetLabel.style.color = "#ecc94b"; // Делаем имя спикера золотым
                }
                break;

            case "reset_speaker":
                SessionState.activeSpeakerId = "";
                document.querySelectorAll('.peer-name, #local-name-label').forEach(el => {
                    el.innerText = el.innerText.replace(" [🎙️ ДОКЛАДЧИК]", "");
                    el.style.color = "white"; // Возвращаем дефолтный белый цвет
                });
                logChat("// [ORCHESTRATION] Статус выделенного спикера аннулирован модератором.", "#64748b");
                break;

            case "force_mute":
                logChat("[ORCHESTRATION] Модератор точечно отключил ваш микрофон.", "#ef4444");
                SessionState.isAudioMuted = true;
                if (SessionState.localStream) SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false);
                const forcedMicBtn = document.getElementById('audio-toggle');
                if (forcedMicBtn) { forcedMicBtn.innerText = "🎤 Микрофон: выкл"; forcedMicBtn.style.backgroundColor = "#7f1d1d"; }
                break;

            case "force_kick":
                alert("Вы были удалены из конференции модератором сессии.");
                window.location.href = "/";
                break;
                            case "toggle_audio_lock":
                if (SessionState.isModerator) break; // Защита Ведущего
                
                const audioBtnNode = document.getElementById('audio-toggle');
                if (!audioBtnNode) break;

                const isAudioLocked = audioBtnNode.style.pointerEvents === "none";

                if (isAudioLocked) {
                    logChat("[ORCHESTRATION] Ведущий снял блокировку звука. Вы можете включить микрофон.", "#10b981");
                    SessionState.isAudioMuted = false;
                    if (SessionState.localStream) SessionState.localStream.getAudioTracks().forEach(t => t.enabled = true);
                    audioBtnNode.innerText = "🎤 Микрофон: вкл";
                    audioBtnNode.style.backgroundColor = "#1e293b";
                    audioBtnNode.style.borderColor = "#334155";
                    audioBtnNode.style.pointerEvents = "auto";
                    audioBtnNode.style.opacity = "1";
                } else {
                    logChat("[ORCHESTRATION] Ведущий заблокировал ваш микрофон (Режим доклада). Включение запрещено.", "#ef4444");
                    SessionState.isAudioMuted = true;
                    if (SessionState.localStream) SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false);
                    audioBtnNode.innerText = "🎤 Микрофон заблокирован";
                    audioBtnNode.style.backgroundColor = "#451a03";
                    audioBtnNode.style.borderColor = "#7c2d12";
                    audioBtnNode.style.pointerEvents = "none";
                    audioBtnNode.style.opacity = "0.5";
                }
                break;

            case "toggle_video_lock":
                if (SessionState.isModerator) break; // Защита Ведущего
                
                const videoBtnNode = document.getElementById('video-toggle');
                const localVideoNode = document.getElementById('local-video');
                if (!videoBtnNode) break;

                const isVideoLocked = videoBtnNode.style.pointerEvents === "none";

                if (isVideoLocked) {
                    logChat("[ORCHESTRATION] Ведущий снял блокировку видеокамер. Вы можете включить видео.", "#10b981");
                    SessionState.isVideoMuted = false;
                    if (SessionState.localStream) SessionState.localStream.getVideoTracks().forEach(t => t.enabled = true);
                    videoBtnNode.innerText = "📷 Камера: вкл";
                    videoBtnNode.style.backgroundColor = "#1e293b";
                    videoBtnNode.style.borderColor = "#334155";
                    videoBtnNode.style.pointerEvents = "auto";
                    videoBtnNode.style.opacity = "1";
                    if (localVideoNode) localVideoNode.style.opacity = "1";
                } else {
                    logChat("[ORCHESTRATION] Ведущий заблокировал вашу видеокамеру (Режим доклада). Включение запрещено.", "#ef4444");
                    SessionState.isVideoMuted = true;
                    if (SessionState.localStream) SessionState.localStream.getVideoTracks().forEach(t => t.enabled = false);
                    videoBtnNode.innerText = "📷 Камера заблокирована";
                    videoBtnNode.style.backgroundColor = "#451a03";
                    videoBtnNode.style.borderColor = "#7c2d12";
                    videoBtnNode.style.pointerEvents = "none";
                    videoBtnNode.style.opacity = "0.5";
                    if (localVideoNode) localVideoNode.style.opacity = "0.15";
                }
                break;
            case "draw_vector_broadcast":
                handleRemoteVectorInjected(msg);
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
    // Клики кнопок админ панели
    setTimeout(() => {
        const muteAllAudioBtn = document.getElementById('mute-all-audio-btn');
        if (muteAllAudioBtn) {
            muteAllAudioBtn.onclick = () => {
                if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                    const currentActiveSpeaker = SessionState.activeSpeakerId || "";
                    SessionState.ws.send(JSON.stringify({
                        type: "control_frame", command: "GLOBAL_MUTE_AUDIO", target_peer_id: currentActiveSpeaker
                    }));
                    const isNowActive = muteAllAudioBtn.classList.toggle('orchestration-active');
                    muteAllAudioBtn.innerText = isNowActive ? "🔇 Звук заблокирован" : "🎙️ Блок звука";
                    muteAllAudioBtn.style.backgroundColor = isNowActive ? "#451a03" : "#1e293b";
                    muteAllAudioBtn.style.borderColor = isNowActive ? "#7c2d12" : "#334155";
                }
            };
        }

        const muteAllVideoBtn = document.getElementById('mute-all-video-btn');
        if (muteAllVideoBtn) {
            muteAllVideoBtn.onclick = () => {
                if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                    const currentActiveSpeaker = SessionState.activeSpeakerId || "";
                    SessionState.ws.send(JSON.stringify({
                        type: "control_frame", command: "GLOBAL_MUTE_VIDEO", target_peer_id: currentActiveSpeaker
                    }));
                    const isNowActive = muteAllVideoBtn.classList.toggle('orchestration-active');
                    muteAllVideoBtn.innerText = isNowActive ? "🔒 Видео заблокировано" : "📷 Блок видео";
                    muteAllVideoBtn.style.backgroundColor = isNowActive ? "#451a03" : "#1e293b";
                    muteAllVideoBtn.style.borderColor = isNowActive ? "#7c2d12" : "#334155";
                }
            };
        }
    }, 1000);

    SessionState.ws.onerror = (err) => { console.error("[SOCKET ERROR]:", err); };
}
