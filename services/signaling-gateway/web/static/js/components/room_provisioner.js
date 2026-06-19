/**
 * handleRoomProvisioning фиксирует введенный ID комнаты и открывает финальный блок инвайт-ссылок
 * ИСПРАВЛЕНО (Живой ввод модератора): Считываем точные значения max-peers и room-duration 
 * и нативно вшиваем их в URL b2b-ссылки, полностью уходя от хардкодных дефолтов!
 * FIXED: Captured live client form inputs to append custom capabilities parameters into organizer token URL
 */
export function handleRoomProvisioning() {
    const roomIdEl = document.getElementById('room-id');
    const maxPeersEl = document.getElementById('max-peers');
    const durationEl = document.getElementById('room-duration');
    
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

    // Считываем живые данные, которые ввёл Давид на экране (с fallback-защитой)
    const maxPeers = maxPeersEl ? (parseInt(maxPeersEl.value, 10) || 100) : 100;
    const duration = durationEl ? (parseInt(durationEl.value, 10) || 30) : 30;

    const host = window.location.host;

    // Сборка b2b-ссылки для модератора Давида со встроенными ЖИВЫМИ лимитами участников и времени Битового Колеса!
    modLinkEl.value = `http://${host}/conference.html?room=${encodeURIComponent(roomId)}&token=header.payload_david_organizer_claims.signature&max_peers=${maxPeers}&duration=${duration}`;
    
    // Сборка инвайт-ссылки для сотрудников компании (EMPLOYEE)
    peerLinkEl.value = `http://${host}/join.html?room=${encodeURIComponent(roomId)}`;

    // Плавно открываем Раздел №3 (Инвайт-ссылки) на интерфейсе
    inviteSection.style.display = "flex";

    // Блокируем поле ввода комнаты, фиксируя конфигурацию шарда
    roomIdEl.disabled = true;
    if (maxPeersEl) maxPeersEl.disabled = true;
    if (durationEl) durationEl.disabled = true;
    document.getElementById('provision-btn-action').disabled = true;
}
