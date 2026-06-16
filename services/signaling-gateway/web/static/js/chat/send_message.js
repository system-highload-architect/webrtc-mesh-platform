import { SessionState } from '../session_context.js';

/**
 * sendMsg перехватывает ввод пользователя, санитизирует строку через REST API и пушит её в WebSocket
 */
export function sendMsg() {
    const input = document.getElementById('chat-input');
    if (!input) return;

    const txt = input.value.trim();
    // Если строка пустая или сокет-соединение не активно — блокируем отправку
    if (!txt || !SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;

    // Сначала отправляем фрейм в REST-эндпоинт бэкенда для XSS-санитизации и AppSec логирования
    fetch(`/api/v1/chat/send?room=${SessionState.roomId}&sender=${SessionState.myPeerId}&text=${encodeURIComponent(txt)}`)
    .then(r => r.text())
    .then(sanitizedText => {
        // Пушим очищенный текст в WebSocket-канал для вещания на весь кластер
        if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
            SessionState.ws.send(JSON.stringify({
                type: "chat",
                room_id: SessionState.roomId,
                sender_name: SessionState.username,
                text: sanitizedText
            }));
        }
    })
    .catch(err => {
        console.error("[AppSec Chat] Крах отправки сообщения через шлюз:", err);
    });

    // Моментально очищаем инпут после отправки
    input.value = "";
}
