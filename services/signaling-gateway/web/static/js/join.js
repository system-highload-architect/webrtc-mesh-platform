/**
 * initJoinPage инициализирует рантайм настройки устройств сотрудников при входе
 * initJoinPage provisions subscriber device mapping before conference lifecycle
 */
export function initJoinPage() {
    const urlParams = new URLSearchParams(window.location.search);
    const roomID = urlParams.get('room');

    if (!roomID) {
        alert("[ОШИБКА] Нарушен периметр ссылки. Отсутствует Room ID.");
        return;
    }

    const joinBtn = document.getElementById('join-btn-action');
    if (!joinBtn) return;

    joinBtn.onclick = () => {
        const email = document.getElementById('employee-email').value;
        const password = document.getElementById('employee-password').value;
        const quality = document.getElementById('video-quality').value;
        const mic = document.getElementById('start-mic').checked;
        const cam = document.getElementById('start-video').checked;

        // ПАТТЕРН БЕЗОПАСНОСТИ (Req. 5): Имитируем b2b-запрос к SPR-хранилищу
        // Валидируем Константина (Роль: EMPLOYEE - Участник сессии)
        if (email === "konstantin@clearway.ru" && password === "user123") {
            const jwtToken = "header.payload_konstantin_employee_claims.signature";
            const host = window.location.host;
            
            // Нативно пробрасываем JWT и медиа-профиль устройств на страницу сессии
            window.location.href = `http://${host}/conference.html?room=${roomID}&token=${jwtToken}&quality=${quality}&mic=${mic}&cam=${cam}`;
        } else {
            alert("❌ Ошибка AppSec: профиль сотрудника в базе SPR не найден или пароль неверен.");
        }
    };
}
