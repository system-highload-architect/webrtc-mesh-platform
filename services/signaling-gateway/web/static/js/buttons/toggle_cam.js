import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

/**
 * toggleVideo переключает состояние локальных видеотреков и обновляет UI-маску превью кадра
 */
export function toggleVideo() {
    // Инвертируем текущее состояние мьюта камеры
    SessionState.isVideoMuted = !SessionState.isVideoMuted;

    // Аппаратно тумблируем видео-треки локального потока в памяти текущей вкладки
    if (SessionState.localStream) {
        SessionState.localStream.getVideoTracks().forEach(track => {
            track.enabled = !SessionState.isVideoMuted;
        });
    }

    // Точечно переключаем стили и текст конкретно этой кнопки нижнего дашборда
    const btn = document.getElementById('video-toggle');
    if (btn) {
        btn.innerText = SessionState.isVideoMuted ? "📷 Камера: выкл" : "📷 Камера: вкл";
        btn.style.backgroundColor = SessionState.isVideoMuted ? "#7f1d1d" : "#1e293b";
        btn.style.borderColor = SessionState.isVideoMuted ? "#ef4444" : "#334155";
    }

    // Точечно управляем прозрачностью только своего видео-тега
    const localVideo = document.getElementById('local-video');
    if (localVideo) {
        localVideo.style.opacity = SessionState.isVideoMuted ? "0.15" : "1";
    }

    logChat(`// [HARDWARE] Состояние локальной видеокамеры изменено: ${SessionState.isVideoMuted ? "MUTED_LOCAL" : "ACTIVE"}`, "#8b949e");
}
