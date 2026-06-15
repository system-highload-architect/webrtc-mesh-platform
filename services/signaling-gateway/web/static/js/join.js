export function initJoinPage() {
    const urlParams = new URLSearchParams(window.location.search);
    const roomID = urlParams.get('room');

    if (!roomID) {
        alert("[ОШИБКА] Отсутствует ID комнаты в инвайт-ссылке периметра.");
        return;
    }

    const joinBtn = document.getElementById('join-btn-action');
    joinBtn.onclick = () => {
        const email = document.getElementById('employee-email').value;
        const password = document.getElementById('employee-password').value;
        const quality = document.getElementById('video-quality').value;
        const mic = document.getElementById('start-mic').checked;
        const cam = document.getElementById('start-video').checked;

        // Паттерн безопасности: отправляем запрос на эмулятор ScyllaDB (SPR)
        // Для локальной демонстрации проверяем каноничную b2b пару сотрудника Константина
        if (email === "konstantin@clearway.ru" && password === "user123") {
            const jwtToken = "header.payload_konstantin_employee_claims.signature";
            const host = window.location.host;
            
            // Нативно пробрасываем JWT и медиа-настройки на страницу сессии
            window.location.href = `http://${host}/conference.html?room=${roomID}&token=${jwtToken}&quality=${quality}&mic=${mic}&cam=${cam}`;
        } else {
            alert("❌ Ошибка авторизации: профиль в базе SPR не найден или пароль неверен.");
        }
    };
}
