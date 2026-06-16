import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

/**
 * toggleAudio переключает состояние (On/Off) локальных звуковых дорожек
 */
export function toggleAudio() {
    // Инвертируем текущее состояние мьюта
    SessionState.isAudioMuted = !SessionState.isAudioMuted;

    // Аппаратно тумблируем аудио-треки нашего локального потока в памяти браузера
    if (SessionState.localStream) {
        SessionState.localStream.getAudioTracks().forEach(track => {
            track.enabled = !SessionState.isAudioMuted;
        });
    }

    // Точечно обновляем визуал и стили конкретно этой кнопки нижнего дашборда
    const btn = document.getElementById('audio-toggle');
    if (btn) {
        btn.innerText = SessionState.isAudioMuted ? "🎤 Микрофон: выкл" : "🎤 Микрофон: вкл";
        btn.style.backgroundColor = SessionState.isAudioMuted ? "#7f1d1d" : "#1e293b";
        btn.style.borderColor = SessionState.isAudioMuted ? "#ef4444" : "#334155";
    }

    logChat(`// [HARDWARE] Состояние локального микрофона изменено: ${SessionState.isAudioMuted ? "MUTED" : "ACTIVE"}`, "#8b949e");
}
