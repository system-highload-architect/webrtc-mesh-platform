let globalJwtToken = "";

/**
 * initIndexPage связывает события b2b-авторизации и генерации комнат
 * initIndexPage binds subscriber auth events and room allocation logic
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

            // Симулируем b2b-проверку паспорта личности в базе SPR
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
            if (!room) {
                alert("Введите уникальный ID комнаты для инициализации.");
                return;
            }

            const host = window.location.host;
            // Прописываем чистые пути к версионированным страницам
            document.getElementById('mod-link').value = `http://${host}/conference.html?room=${room}&token=${globalJwtToken}`;
            document.getElementById('peer-link').value = `http://${host}/join.html?room=${room}`;
            document.getElementById('invite-section').style.display = "block";
        };
    }

    if (enterModBtn) {
        enterModBtn.onclick = () => {
            const link = document.getElementById('mod-link').value;
            if (link) window.location.href = link;
        };
    }
}
