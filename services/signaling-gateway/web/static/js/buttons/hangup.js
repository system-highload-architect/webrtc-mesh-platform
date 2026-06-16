import { SessionState } from '../session_context.js';

/**
 * hangUp останавливает захват медиа-треков, закрывает сокет и возвращает ноду в корень
 */
export function hangUp() {
    // 1. Аппаратно тушим треки локальной веб-камеры и микрофона из RAM
    if (SessionState.localStream) {
        SessionState.localStream.getTracks().forEach(track => {
            track.stop();
        });
        SessionState.localStream = null;
    }

    // 2. Аппаратно тушим активные треки демонстрации экрана из RAM
    if (SessionState.screenStream) {
        SessionState.screenStream.getTracks().forEach(track => {
            track.stop();
        });
        SessionState.screenStream = null;
    }

    // 3. Закрываем нативные P2P WebRTC соединения со всеми Mesh-нодами
    for (let peerId in SessionState.peerConnections) {
        if (SessionState.peerConnections[peerId]) {
            SessionState.peerConnections[peerId].close();
            delete SessionState.peerConnections[peerId];
        }
    }

    // 4. Корректно закрываем полнодуплексный сокет-канал Gateway
    if (SessionState.ws) {
        SessionState.ws.close();
        SessionState.ws = null;
    }

    // Возвращаем пользователя на стартовую страницу
    window.location.href = "/";
}
