import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

/**
 * executeRemoteMuteTargeted принудительно глушит конкретного выбранного сотрудника по ID через Бэк
 */
export function executeRemoteMuteTargeted(peerId) {
    if (!SessionState.isModerator || !SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;

    logChat(`// [ORCHESTRATION] Отправлена Бэкенд-директива: MUTE -> [${peerId}]`, "#ecc94b");

    // ИСПРАВЛЕНО: Шлем плоскую структуру для мгновенного парсинга в Go
    SessionState.ws.send(JSON.stringify({
        type: "control_frame",
        command: "MUTE_AUDIO",
        target_peer_id: peerId
    }));
}

/**
 * executeRemoteKickTargeted принудительно удаляет выбранного пира из Mesh-контура комнаты
 */
export function executeRemoteKickTargeted(peerId) {
    if (!SessionState.isModerator || !SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;

    logChat(`// [ORCHESTRATION] Выдан Бэкенд-KICK -> [${peerId}]`, "#ef4444");

    SessionState.ws.send(JSON.stringify({
        type: "control_frame",
        command: "KICK_PEER",
        target_peer_id: peerId
    }));
}
