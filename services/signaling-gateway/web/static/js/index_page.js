import { handleOrganizerLogin } from './components/auth_handler.js';
import { handleRoomProvisioning } from './components/room_provisioner.js';
import { setupLinkInteraction } from './components/link_generator.js';

/**
 * initIndexPage связывает b2b-компоненты управления с DOM-нодами страницы index.html
 */
function initIndexPage() {
    const loginBtn = document.getElementById('login-btn-action');
    if (loginBtn) {
        loginBtn.onclick = handleOrganizerLogin;
    }

    const provisionBtn = document.getElementById('provision-btn-action');
    if (provisionBtn) {
        provisionBtn.onclick = handleRoomProvisioning;
    }

    // ИСПРАВЛЕНО: Замыкаем контур, активируя нативное клик-копирование и редирект
    // FIXED: Invoked decoupled clipboard link interaction pipeline listeners
    setupLinkInteraction();
}

// Запускаем инициализацию страницы
initIndexPage();
