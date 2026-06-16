import { captureLobbyPreview } from './components/lobby_media.js';
import { toggleLobbyAudio, toggleLobbyVideo } from './components/lobby_controls.js';
import { setupLobbyRouter } from './components/lobby_router.js';

let lobbyLocalStream = null;

/**
 * initJoinPage инициализирует изолированный рантайм лобби-экрана сотрудника (EMPLOYEE)
 */
async function initJoinPage() {
    const urlParams = new URLSearchParams(window.location.search);
    const roomId = urlParams.get('room') || "default-room";

    const roomIdEl = document.getElementById('lobby-room-id');
    if (roomIdEl) {
        roomIdEl.innerText = `Room Token: ${roomId}`;
    }

    // Аппаратный захват веб-камеры и микрофона
    lobbyLocalStream = await captureLobbyPreview();
    window.lobbyStreamRef = lobbyLocalStream;

    const audioBtn = document.getElementById('lobby-audio-toggle');
    const videoBtn = document.getElementById('lobby-video-toggle');

    // ИСПРАВЛЕНО (Устранение ReferenceError): Привязываем чистый изолированный метод без опечаток
    if (audioBtn) audioBtn.onclick = toggleLobbyAudio;
    if (videoBtn) videoBtn.onclick = toggleLobbyVideo;

    // Инициализируем финальный обработчик роутинга и клика по кнопке "Войти"
    setupLobbyRouter(roomId);
}

initJoinPage();
