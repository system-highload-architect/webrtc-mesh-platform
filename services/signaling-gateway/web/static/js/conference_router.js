import { SessionState } from './session_context.js';
import { initSocketConnection } from './services/socket_service.js';
import { captureLocalMedia } from './services/media_capture.js';
import { bindDashboardEvents } from './events/dashboard_binder.js';

/**
 * routeConferenceSession запускает пошаговую b2b-оркестрацию распределенных JS-модулей
 */
async function routeConferenceSession() {
    const urlParams = new URLSearchParams(window.location.search);
    
    // Парсим персистентные конфигурационные параметры из URL-строки перехода
    SessionState.roomId = urlParams.get('room') || "demo_room";
    const tokenStr = urlParams.get('token') || "";
    SessionState.isModerator = tokenStr.includes("david_organizer");
    
    // Извлекаем max_peers и duration из адресной строки перехода
    const maxPeers = urlParams.get('max_peers') || "100";
    const duration = urlParams.get('duration') || "30";
    
    // Считываем уникальный Peer ID ноды, сгенерированный на этапе лобби-контроля
    SessionState.myPeerId = urlParams.get('peer') || (SessionState.isModerator ? "David_Moderator" : "User_Guest");
    SessionState.username = SessionState.myPeerId;

    // Выводим ID комнаты на интерфейс статус-бара
    const roomLbl = document.getElementById('room-lbl');
    if (roomLbl) {
        roomLbl.innerText = SessionState.roomId;
    }

    // Выводим Claim-роль участника на текстовый значок
    const roleBadge = document.getElementById('role-badge');
    if (roleBadge) {
        if (SessionState.isModerator) {
            roleBadge.innerText = "👑 МОДЕРАТОР СЕССИИ (ORGANIZER)";
            roleBadge.style.color = "#319795";
            
            // Нативно открываем пульт модерации Давида, инжектированный Go-шаблонизатором
            const modControls = document.getElementById('moderator-controls');
            if (modControls) modControls.style.display = "flex";
        } else {
            roleBadge.innerText = "📱 СОТРУДНИК ПЛАТФОРМЫ (EMPLOYEE)";
            roleBadge.style.color = "#4299e1";
        }
    }

    // 1. Асинхронно запускаем аппаратный захват веб-камеры и микрофона со стейтами лобби
    await captureLocalMedia();

    // 2. Включаем полнодуплексный сокет-канал Gateway к API Gateway балансировщику
    initSocketConnection(maxPeers, duration);

    // 3. Биндим чистые изолированные Event Listeners на кнопки дашборда управления треками
    bindDashboardEvents();
}

// Запускаем рантайм комнаты конференции
routeConferenceSession();
