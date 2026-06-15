let globalJwtToken = "";

/**
 * initIndexPage инициализирует b2b-клиентскую логику главной панели управления
 * initIndexPage provisions root dashboard click captures and invite payload builders
 */
function initIndexPage() {
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

            // Имитируем b2b-проверку паспорта личности (взаимодействие со слоем SPR/Auth)
            if (email === "david@clearway.ru" && password === "admin123") {
                globalJwtToken = "header.payload_david_organizer_claims.signature";
                if (statusEl) {
                    statusEl.innerText = "// STATE: Успешно авторизован -> Владелец (ORGANIZER)";
                    statusEl.style.color = "#319795";
                }
                const setupSec = document.getElementById('setup-section');
                if (setupSec) setupSec.style.display = "block";
            } else if (email === "konstantin@clearway.ru" && password === "user123") {
                globalJwtToken = "header.payload_konstantin_employee_claims.signature";
                if (statusEl) {
                    statusEl.innerText = "// STATE: Успешно авторизован -> Сотрудник (EMPLOYEE)";
                    statusEl.style.color = "#4299e1";
                }
                const setupSec = document.getElementById('setup-section');
                if (setupSec) setupSec.style.display = "block";
            } else {
                if (statusEl) {
                    statusEl.innerText = "// STATE: ❌ Ошибка AppSec: неверный Email или Пароль";
                    statusEl.style.color = "#e53e3e";
                }
            }
        };
    }

    if (provisionBtn) {
        provisionBtn.onclick = () => {
            const room = document.getElementById('room-id').value;
            if (!room) {
                alert("Укажите уникальный ID комнаты для инициализации RAM-шарда.");
                return;
            }

            const host = window.location.host;
            
            // Распределяем пути роутинга по изолированным страницам-акцепторам
            const modLinkEl = document.getElementById('mod-link');
            const peerLinkEl = document.getElementById('peer-link');
            const inviteSec = document.getElementById('invite-section');

            if (modLinkEl) modLinkEl.value = `http://${host}/conference.html?room=${room}&token=${globalJwtToken}`;
            if (peerLinkEl) peerLinkEl.value = `http://${host}/join.html?room=${room}`;
            if (inviteSec) inviteSec.style.display = "block";
        };
    }

    if (enterModBtn) {
        enterModBtn.onclick = () => {
            const modLinkEl = document.getElementById('mod-link');
            if (modLinkEl && modLinkEl.value) {
                window.location.href = modLinkEl.value; // Перенаправляем Владельца на контур конференции
            }
        };
    }

    // ФИЧА (ГОТОВО): Нативное автокопирование в буфер обмена по клику на инпуты ссылок (UX Лоск)
    // FIXED: Embedded instantaneous clipboard copy-on-click trigger for seamless onboarding experience
    setupClipboardClickCopy('mod-link', '👑 Ссылка модератора скопирована в буфер обмена');
    setupClipboardClickCopy('peer-link', '📱 Ссылка сотрудника скопирована в буфер обмена');
}

/**
 * setupClipboardClickCopy вешает b2b-интерцептор клика для автокопирования
 */
function setupClipboardClickCopy(elementId, successMessage) {
    const el = document.getElementById(elementId);
    if (!el) return;

    el.style.cursor = "copy"; // Меняем курсор на иконку копирования при наведении

    el.onclick = () => {
        if (!el.value) return;

        // Пользуемся нативным асинхронным API браузера для записи в буфер обмена хоста
        navigator.clipboard.writeText(el.value)
        .then(() => {
            const originalColor = el.style.color;
            const originalValue = el.value;

            // Нативно подсвечиваем инпут неоновым amber цветом в момент успеха
            el.style.color = "#ecc94b";
            el.value = `// ✅ СКОПИРОВАНО: ${successMessage.split(' скопирована')[0]}`;
            
            // Лениво возвращаем исходный текст ссылки на место через 1.5 секунды
            setTimeout(() => {
                el.style.color = originalColor;
                el.value = originalValue;
            }, 1500);
        })
        .catch(err => {
            console.error("[UX Error] Не удалось записать данные в буфер обмена:", err);
        });
    };
}

// Автоматически запускаем инициализацию при загрузке модуля в контексте страницы
initIndexPage();
