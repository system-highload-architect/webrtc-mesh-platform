import { SessionState } from '../session_context.js';

/**
 * captureLocalMedia выполняет аппаратный захват аудио и видео треков операционной системы
 */
export async function captureLocalMedia() {
    try {
        const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
        SessionState.localStream = stream;

        const localVid = document.getElementById('local-video');
        if (localVid) {
            localVid.srcObject = stream;
            localVid.style.opacity = "1";
        }
    } catch (e) {
        console.warn("[HARDWARE LAYER] Камера заблокирована или отсутствует:", e.message);
        
        // Фолбэк-заглушка для UI во избежание падения контура верстки при тестах на серверах без камер
        const container = document.getElementById('local-video-container');
        if (container) {
            container.style.backgroundColor = "#090d16";
            container.innerHTML += `<span style="color:#ef4444; font-size:10px; font-family:monospace; position:absolute; z-index:2;">⚠️ NO HARDWARE DETECTED</span>`;
        }
    }
}
