/**
 * handleRoomProvisioning фиксирует введенный ID комнаты и открывает финальный блок инвайт-ссылок
 */
export function handleRoomProvisioning() {
    const roomIdEl = document.getElementById('room-id');
    const inviteSection = document.getElementById('invite-section');
    const modLinkEl = document.getElementById('mod-link');
    const peerLinkEl = document.getElementById('peer-link');

    if (!roomIdEl || !inviteSection || !modLinkEl || !peerLinkEl) return;

    const roomId = roomIdEl.value.trim();

    // Защита: ID комнаты не должен быть пустым по ТЗ
    if (!roomId) {
        roomIdEl.style.borderColor = "#ef4444";
        alert("Пожалуйста, укажите уникальный ID создаваемой b2b-комнаты.");
        return;
    }

    const host = window.location.host;

    // Сборка b2b-ссылки для модератора Давида со встроенным токеном роли (ORGANIZER)
    modLinkEl.value = `http://${host}/conference.html?room=${encodeURIComponent(roomId)}&token=header.payload_david_organizer_claims.signature`;
    
    // Сборка инвайт-ссылки для сотрудников компании (EMPLOYEE)
    peerLinkEl.value = `http://${host}/join.html?room=${encodeURIComponent(roomId)}`;

    // Плавно открываем Раздел №3 (Инвайт-ссылки) на интерфейсе
    inviteSection.style.display = "flex";

    // Блокируем поле ввода комнаты, фиксируя конфигурацию шарда
    roomIdEl.disabled = true;
    document.getElementById('provision-btn-action').disabled = true;
}
