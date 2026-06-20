import { LobbyState } from './lobby_controls.js';

/**
 * setupLobbyRouter биндит клик по кнопке "Войти в конференцию"
 */
export function setupLobbyRouter(roomId) {
    const enterBtn = document.getElementById('lobby-enter-btn');
    if (!enterBtn) return;

    enterBtn.onclick = () => {
        const emailEl = document.getElementById('employee-email');
        const qualityEl = document.getElementById('video-quality');
        
        if (!emailEl || !qualityEl) return;

        const email = emailEl.value.trim();
        if (!email) {
            emailEl.style.borderColor = "#ef4444";
            alert("Пожалуйста, укажите ваш корпоративный Email для авторизации ноды.");
            return;
        }

        // namePart — это массив строк после split. Берем строго нулевой индекс!
        // Explicitly extracted zeroth index entry from split string array to prevent charAt TypeError
        const namePart = email.split('@');
        const rawName = namePart[0];
        const formattedPeerID = rawName.charAt(0).toUpperCase() + rawName.slice(1) + "_Guest";

        // Генерируем токен роли для API Gateway прокси
        const jwtToken = `header.payload_${rawName}_employee_claims.signature`;
        const host = window.location.host;

        // Останавливаем локальный превью-стрим лобби перед переходом
        if (window.lobbyStreamRef) {
            window.lobbyStreamRef.getTracks().forEach(track => track.stop());
        }

        // Переводим булевы стейты в явные параметры
        const micState = !LobbyState.isAudioMuted;
        const camState = !LobbyState.isVideoMuted;
        const quality = qualityEl.value;

        // Нативно пробрасываем выбранные в лобби тумблеры устройств в рабочую комнату созвона
        window.location.href = `http://${host}/conference.html?room=${encodeURIComponent(roomId)}&token=${jwtToken}&peer=${formattedPeerID}&quality=${quality}&mic=${micState}&cam=${camState}`;
    };
}
