import { SessionState } from '../session_context.js';

/**
 * captureLocalMedia выполняет аппаратный захват сопряженных аудио/видео треков на базе селектора качества
 */
export async function captureLocalMedia() {
    const urlParams = new URLSearchParams(window.location.search);
    
    SessionState.isAudioMuted = urlParams.get('mic') === "false";
    SessionState.isVideoMuted = urlParams.get('cam') === "false";

    // Считываем выбранное в лобби качество и выставляем constraints
    // Dynamically resolved hardware video resolution bounds from the incoming quality slug
    const qualitySlug = urlParams.get('quality') || "720p";
    
    let targetWidth = 1280;
    let targetHeight = 720;

    if (qualitySlug === "480p") {
        targetWidth = 854;
        targetHeight = 480;
    } else if (qualitySlug === "360p") {
        targetWidth = 640;
        targetHeight = 360;
    }

    try {
        // Нативно запрашиваем точные b2b-разрешения у операционной системы ПК
        const stream = await navigator.mediaDevices.getUserMedia({ 
            video: { 
                width: { ideal: targetWidth }, 
                height: { ideal: targetHeight },
                frameRate: { ideal: 30 }
            }, 
            audio: true 
        });
        SessionState.localStream = stream;

        // Аппаратно синхронизируем дорожки
        stream.getAudioTracks().forEach(track => { track.enabled = !SessionState.isAudioMuted; });
        stream.getVideoTracks().forEach(track => { track.enabled = !SessionState.isVideoMuted; });

        const localVid = document.getElementById('local-video');
        if (localVid) {
            localVid.srcObject = stream;
            localVid.style.opacity = SessionState.isVideoMuted ? "0.15" : "1";
        }

        const audioBtn = document.getElementById('audio-toggle');
        const videoBtn = document.getElementById('video-toggle');
        
        if (audioBtn) audioBtn.innerText = SessionState.isAudioMuted ? "🎤 Микрофон: выкл" : "🎤 Микрофон: вкл";
        if (videoBtn) videoBtn.innerText = SessionState.isVideoMuted ? "📷 Камера: выкл" : "📷 Камера: вкл";

    } catch (e) {
        console.warn("[HARDWARE LAYER] Камера заблокирована или отсутствует:", e.message);
        const container = document.getElementById('local-video-container');
        if (container) {
            container.style.backgroundColor = "#090d16";
            container.innerHTML += `<span style="color:#ef4444; font-size:10px; font-family:monospace; position:absolute; z-index:2;">⚠️ NO HARDWARE DETECTED</span>`;
        }
    }
}
