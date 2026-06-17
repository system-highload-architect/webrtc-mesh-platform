import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

export function executeGlobalVideoBlock() {
    if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
        const currentActiveSpeaker = SessionState.activeSpeakerId || "";

        SessionState.ws.send(JSON.stringify({
            type: "control_frame",
            command: "GLOBAL_MUTE_VIDEO",
            target_peer_id: currentActiveSpeaker // Передаем ID спикера на бэк для защиты
        }));
        logChat("// [ORCHESTRATION] Сигнал GLOBAL_MUTE_VIDEO отправлен в контур сессии.", "#ef4444");
    }
}
