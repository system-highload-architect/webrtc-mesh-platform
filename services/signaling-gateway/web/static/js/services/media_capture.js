import { SessionState } from '../session_context.js';

/**
 * captureLocalMedia выполняет аппаратный захват аудио и видео треков операционной системы
 */
export async function captureLocalMedia() {
    const urlParams = new URLSearchParams(window.location.search);
    
    // Считываем проброшенные из лобби стейты кнопок (по умолчанию true)
    SessionState.isAudioMuted = urlParams.get('mic') === "false";
    SessionState.isVideoMuted = urlParams.get('cam') === "false";

    try {
        const stream = await navigator.mediaDevices.getUserMedia({ 
            video: { width: 1280, height: 720 }, 
            audio: true 
        });
        SessionState.localStream = stream;

        // Аппаратно синхронизируем дорожки звука со стейтом лобби
        stream.getAudioTracks().forEach(track => {
            track.enabled = !SessionState.isAudioMuted;
        });

        // Аппаратно синхронизируем дорожки видео со стейтом лобби
        stream.getVideoTracks().forEach(track => {
            track.enabled = !SessionState.isVideoMuted;
        });

        const localVid = document.getElementById('local-video');
        if (localVid) {
            localVid.srcObject = stream;
            // Устанавливаем маску прозрачности видео встык со стейтом лобби
            localVid.style.opacity = SessionState.isVideoMuted ? "0.15" : "1";
        }

        // Синхронизируем текст и цвет кнопок нижнего пульта со стейтом лобби
        const audioBtn = document.getElementById('audio-toggle');
        const videoBtn = document.getElementById('video-toggle');
        
        if (audioBtn) {
            audioBtn.innerText = SessionState.isAudioMuted ? "🎤 Микрофон: выкл" : "🎤 Микрофон: вкл";
            audioBtn.style.backgroundColor = SessionState.isAudioMuted ? "#7f1d1d" : "#1e293b";
        }
        if (videoBtn) {
            videoBtn.innerText = SessionState.isVideoMuted ? "📷 Камера: выкл" : "📷 Камера: вкл";
            videoBtn.style.backgroundColor = SessionState.isVideoMuted ? "#7f1d1d" : "#1e293b";
        }

    } catch (e) {
        console.warn("[HARDWARE LAYER] Камера заблокирована или отсутствует:", e.message);
        const container = document.getElementById('local-video-container');
        if (container) {
            container.style.backgroundColor = "#090d16";
            container.innerHTML += `<span style="color:#ef4444; font-size:10px; font-family:monospace; position:absolute; z-index:2;">⚠️ NO HARDWARE DETECTED</span>`;
        }
    }
}
