/**
 * initJoinPage обеспечивает перехват b2b-авторизации сотрудников и проброс медиа-профилей
 * initJoinPage provisions subscriber entry point clicks and dynamic multi-peer session identifiers
 */
export function initJoinPage() {
    const urlParams = new URLSearchParams(window.location.search);
    const roomID = urlParams.get('room');

    if (!roomID) {
        alert("[ОШИБКА периметра]: Нарушен контур инвайт-ссылки. Отсутствует Room ID комнат.");
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

        if (!email || !password) {
            alert("Заполните паспорт учетной записи сотрудника.");
            return;
        }

        // ИСПРАВЛЕНО: Динамически вычленяем имя из Email для генерации множества уникальных участников кластера
        // FIXED: Dynamically extract name string from credentials block to prevent single socket collision locks
        const namePart = email.split('@')[0];
        const formattedPeerID = namePart.charAt(0).toUpperCase() + namePart.slice(1) + "_Guest";

        const jwtToken = `header.payload_${namePart}_employee_claims.signature`;
        const host = window.location.host;
        
        // Перенаправляем на conference.html с пробросом уникального Peer ID
        window.location.href = `http://${host}/conference.html?room=${roomID}&token=${jwtToken}&peer=${formattedPeerID}&quality=${quality}&mic=${mic}&cam=${cam}`;
    };
}
