import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

/**
 * executeRemoteMute шлет в сокет административную команду принудительного глушения звука пира
 */
export function executeRemoteMute() {
    // Безопасность: выполнять команду может только верифицированный модератор сессии
    if (!SessionState.isModerator) return;

    if (!SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) {
        console.warn("[ORCHESTRATION] Сигнальный туннель не активен.");
        return;
    }

    logChat("// [ORCHESTRATION] Инициализирована широковещательная команда: MUTE_AUDIO -> User_Guest", "#ecc94b");

    // Выстреливаем управляющий b2b-фрейм модерации в WebSocket-шину кластера
    SessionState.ws.send(JSON.stringify({
        type: "control_frame",
        payload: btoa(JSON.stringify({ 
            command: "MUTE_AUDIO", 
            target_peer_id: "User_Guest" // Направляем директиву на дефолтного гостя
        }))
    }));
}
