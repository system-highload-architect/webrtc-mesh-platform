import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

export function executeGlobalAudioBlock() {
    if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
        // Вычисляем, кто сейчас спикер (если стейт пустой - защищен только Давид)
        const currentActiveSpeaker = SessionState.activeSpeakerId || "";

        SessionState.ws.send(JSON.stringify({
            type: "control_frame",
            command: "GLOBAL_MUTE_AUDIO",
            target_peer_id: currentActiveSpeaker // Передаем ID спикера на бэк для защиты
        }));
        logChat("// [ORCHESTRATION] Сигнал GLOBAL_MUTE_AUDIO отправлен в контур сессии.", "#ecc94b");
    }
}
