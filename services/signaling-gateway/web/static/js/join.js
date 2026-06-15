let globalJwtToken = "";

/**
 * initIndexPage инициализирует b2b-клиентскую логику главной панели управления
 * initIndexPage provisions root dashboard click captures and invite payload builders
 */
export function initIndexPage() {
    const loginBtn = document.getElementById('login-btn-action');
    const provisionBtn = document.getElementById('provision-btn-action');
    const enterModBtn = document.getElementById('enter-mod-btn-action');

    if (loginBtn) {
        loginBtn.onclick = () => {
            const email = document.getElementById('auth-email').value;
            const password = document.getElementById('auth-password').value;
            const statusEl = document.getElementById('auth-status');

            if (!email || !password) {
                alert("Заполните учетные данные для верификации личности.");
                return;
            }

            // Имитируем b2b-проверку паспорта личности. Сервер вернет токен
            if (email === "david@clearway.ru" && password === "admin123") {
                globalJwtToken = "header.payload_david_organizer_claims.signature";
                statusEl.innerText = "✅ Авторизован: Давид (Роль: ORGANIZER)";
                statusEl.style.color = "#319795";
                document.getElementById('setup-section').style.display = "block";
            } else if (email === "konstantin@clearway.ru" && password === "user123") {
                globalJwtToken = "header.payload_konstantin_employee_claims.signature";
                statusEl.innerText = "✅ Авторизован: Константин (Роль: EMPLOYEE)";
                statusEl.style.color = "#4299e1";
                document.getElementById('setup-section').style.display = "block";
            } else {
                statusEl.innerText = "❌ Ошибка AppSec: неверный Email или Пароль";
                statusEl.style.color = "#e53e3e";
            }
        };
    }

    if (provisionBtn) {
        provisionBtn.onclick = () => {
            const room = document.getElementById('room-id').value;
            const maxPeers = document.getElementById('max-peers').value;
            const duration = document.getElementById('duration').value;
            const emails = document.getElementById('invite-emails').value;

            if (!room) {
                alert("Укажите уникальный ID комнаты для инициализации RAM-шарда.");
                return;
            }

            const host = window.location.host;
            
            // Нативно распределяем пути роутинга по изолированным страницам (Multi-Page Architecture)
            document.getElementById('mod-link').value = `http://${host}/conference.html?room=${room}&token=${globalJwtToken}`;
            document.getElementById('peer-link').value = `http://${host}/join.html?room=${room}`;
            
            document.getElementById('invite-section').style.display = "block";
        };
    }

    if (enterModBtn) {
        enterModBtn.onclick = () => {
            const link = document.getElementById('mod-link').value;
            if (link) {
                window.location.href = link; // Перенаправляем Владельца на холст конференции
            }
        };
    }
}
