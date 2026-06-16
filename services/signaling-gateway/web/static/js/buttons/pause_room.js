import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';
import { stopScreenShare } from './screen_share.js';

/**
 * togglePauseRoomSignal инвертирует стейт паузы комнаты и пушит директиву на Бэкенд
 */
export function togglePauseRoomSignal() {
    if (!SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;
    
    SessionState.isPaused = !SessionState.isPaused;
    const pauseBtn = document.getElementById('pause-btn-action');
    
    if (pauseBtn) {
        if (SessionState.isPaused) {
            // ИСПРАВЛЕНО: Шлем плоскую структуру команды напрямую в сокет бэка
            SessionState.ws.send(JSON.stringify({ 
                type: "control_frame", 
                command: "SET_PAUSE" 
            }));
            pauseBtn.innerText = "▶️ Снять Паузу";
            pauseBtn.style.borderColor = "#319795"; pauseBtn.style.color = "#319795";
        } else {
            SessionState.ws.send(JSON.stringify({ 
                type: "control_frame", 
                command: "RESUME_CONFERENCE" 
            }));
            pauseBtn.innerText = "⏸️ Пауза";
            pauseBtn.style.borderColor = "#ecc94b"; pauseBtn.style.color = "#ecc94b";
        }
    }
}

/**
 * toggleUiFreeze наглухо, аппаратно тушит девайсы сотрудников, оставляя ЧАТ открытым
 */
export function toggleUiFreeze(freeze) {
    SessionState.isPaused = freeze;

    const wrappers = document.querySelectorAll('.video-wrapper');
    wrappers.forEach(w => w.classList.toggle('room-frozen', freeze));

    if (freeze) {
        if (SessionState.isScreenSharing) {
            stopScreenShare();
        }

        if (SessionState.localStream) {
            SessionState.localStream.getAudioTracks().forEach(t => t.enabled = false);
            SessionState.localStream.getVideoTracks().forEach(t => t.enabled = false);
        }

        const localVid = document.getElementById('local-video');
        if (localVid) localVid.style.opacity = "0.15";

        const audioBtn = document.getElementById('audio-toggle');
        const videoBtn = document.getElementById('video-toggle');
        if (audioBtn) { audioBtn.innerText = "🎤 Микрофон: блокирован"; audioBtn.style.backgroundColor = "#7f1d1d"; }
        if (videoBtn) { videoBtn.innerText = "📷 Камера: блокирована"; videoBtn.style.backgroundColor = "#7f1d1d"; }

        logChat("// [ORCHESTRATION] Сессия ЗАМОРОЖЕНА модератором. Камеры и микрофоны аппаратно ОТКЛЮЧЕНЫ. Чат активен.", "#ef4444");
    } else {
        if (SessionState.localStream) {
            SessionState.localStream.getAudioTracks().forEach(t => t.enabled = !SessionState.isAudioMuted);
            SessionState.localStream.getVideoTracks().forEach(t => t.enabled = !SessionState.isVideoMuted);
        }

        const localVid = document.getElementById('local-video');
        if (localVid) localVid.style.opacity = SessionState.isVideoMuted ? "0.15" : "1";

        const audioBtn = document.getElementById('audio-toggle');
        const videoBtn = document.getElementById('video-toggle');
        if (audioBtn) { audioBtn.innerText = SessionState.isAudioMuted ? "🎤 Микрофон: выкл" : "🎤 Микрофон: вкл"; audioBtn.style.backgroundColor = SessionState.isAudioMuted ? "#7f1d1d" : "#1e293b"; }
        if (videoBtn) { videoBtn.innerText = SessionState.isVideoMuted ? "📷 Камера: выкл" : "📷 Камера: вкл"; videoBtn.style.backgroundColor = SessionState.isVideoMuted ? "#7f1d1d" : "#1e293b"; }

        logChat("// [ORCHESTRATION] Заморозка снята модератором Давидом. Устройства разблокированы.", "#10b981");
    }
    
    const buttons = document.querySelectorAll('.ctrl-btn');
    buttons.forEach(btn => { 
        if (btn.id !== "hangup-btn" && btn.id !== "chat-send-btn") {
            btn.disabled = freeze; 
        }
    });
}
