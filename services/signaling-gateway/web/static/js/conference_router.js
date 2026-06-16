import { SessionState } from './session_context.js';
import { initSocketConnection } from './services/socket_service.js';
import { captureLocalMedia } from './services/media_capture.js';
import { bindDashboardEvents } from './events/dashboard_binder.js';

/**
 * routeConferenceSession запускает оркестрацию распределенных JS-модулей
 */
async function routeConferenceSession() {
    const urlParams = new URLSearchParams(window.location.search);
    
    SessionState.roomId = urlParams.get('room') || "demo_room";
    const tokenStr = urlParams.get('token') || "";
    SessionState.isModerator = tokenStr.includes("david_organizer");
    
    // Динамически выставляем уникальный ID пира в рамках Mesh-контура
    SessionState.myPeerId = urlParams.get('peer') || (SessionState.isModerator ? "David_Moderator" : `Peer_${Math.floor(Math.random() * 1000)}_Guest`);
    SessionState.username = SessionState.myPeerId;

    // Выводим ID комнаты на интерфейс в контейнер-акцептор
    const roomLbl = document.getElementById('room-lbl');
    if (roomLbl) roomLbl.innerText = SessionState.roomId;

    // Инициализируем аппаратный слой виртуальных или реальных медиа-треков
    await captureLocalMedia();

    // Включаем сокет-подключение плоскости управления к API Gateway
    initSocketConnection();

    // Биндим события на все кнопки нижнего и бокового пультов
    bindDashboardEvents();
}

// Запускаем маршрутизатор сессии
routeConferenceSession();
