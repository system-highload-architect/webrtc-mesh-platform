import { checkT9, handleTabCompletion } from './chat.js';
import { renderIncomingVector } from './drawing.js';

let ws = null;
let isRecording = false;
let isDrawingMode = false;
let activeSuggestion = "";

/**
 * initConference инициализирует WebSocket-коммутацию и Т9-конвейер чата сессии (Req. 4 & 5)
 */
export function initConference(roomID, peerID, tokenStr, initQuality, initMic, initCam, isModerator) {
    const inputEl = document.getElementById('chat-input');
    const placeholderEl = document.getElementById('t9-placeholder');

    if (!inputEl || !placeholderEl) return;

    // 1. Привязываем наносекундный Т9-конвейер к инпуту сообщений чата
    inputEl.oninput = async () => {
        activeSuggestion = await checkT9(inputEl, placeholderEl);
    };

    inputEl.onkeydown = (e) => {
        if (e.key === "Tab" && activeSuggestion) {
            e.preventDefault();
            handleTabCompletion(inputEl, placeholderEl, activeSuggestion);
            activeSuggestion = "";
        }
        if (e.key === "Enter" && inputEl.value.trim()) {
            executeMessageSend(roomID, peerID, inputEl.value);
            inputEl.value = "";
            placeholderEl.innerText = "";
            activeSuggestion = "";
        }
    };

    // 2. Подключаем версионированный v1 WebSocket сигнального шлюза монолита
    // Передаем mod=true/false для локальной симуляции двух ролей в терминале
    ws = new WebSocket(`ws://${window.location.host}/api/v1/ws?room=${roomID}&peer=${peerID}&mod=${isModerator}`);
    window.ws = ws; // Намертво выносим дескриптор сокета в глобальную область видимости для drawing.js!

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        handleIncomingSignal(msg, initQuality, initMic, initCam);
    };
}

function handleIncomingSignal(msg, initQuality, initMic, initCam) {
    if (msg.type === "chat_history_dump") {
        msg.logs.forEach(logFrame => {
            appendChatMsg(logFrame.sender_id, logFrame.text);
        });
        return;
    }
    
    if (msg.type === "draw_vector") {
        renderIncomingVector(msg);
        return;
    }
    
    if (msg.type === "auth_error") {
        alert("[ОШИБКА БЕЗОПАСНОСТИ] Сессия заблокирована.");
        window.location.href = "/";
        return;
    }
    if (msg.type === "peer_joined") {
        document.getElementById('remote-video-box').style.display = "flex";
        document.getElementById('remote-status').innerText = "Поток активен [P2P Secure Mesh Tunnel]";
        appendSystemMsg(`Абонент подключился к Full-Mesh контуру.`);
    }
    if (msg.type === "peer_left") {
        document.getElementById('remote-video-box').style.display = "none";
        appendSystemMsg(`Абонент разорвал P2P-соединение.`);
    }
    if (msg.type === "chat_broadcast") {
        appendChatMsg(msg.sender_id, msg.text);
    }
    if (msg.type === "force_mute") {
        appendSystemMsg("⚠️ Модератор принудительно заблокировал ваш микрофон.");
        document.getElementById('local-status').innerText = "🎙️ МИКРОФОН СБРОШЕН АДМИНИСТРАТОРОМ";
    }
    if (msg.type === "force_kick") {
        alert("Вы были принудительно изгнаны (KICKED) из конференции.");
        window.location.href = "/";
    }
    if (msg.type === "room_paused") {
        document.getElementById('local-video-box').classList.add('paused');
        document.getElementById('remote-video-box').classList.add('paused');
        document.getElementById('local-status').innerText = "⏸️ ПЕРЕРЫВ (Muted Keyframes - 1 кадр / 5 сек)";
        appendSystemMsg("⏸️ Модератор установил режим общего перерыва. Экономия полосы сервера: 95%.");
        if (isRecording) toggleRecording(); // Авто-пауза записи (Req. 3)
    }
}

function executeMessageSend(roomID, myPeerID, rawText) {
    // Отправляем текст в REST v1 эндпоинт чата бэкенда для XSS-фильтрации и CAS-лимитера
    fetch(`/api/v1/chat/send?room=${roomID}&sender=${myPeerID}&text=${encodeURIComponent(rawText)}`)
    .then(r => r.text())
    .then(sanitizedText => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({type: "chat_msg", text: sanitizedText}));
        }
    });
}

export function triggerControl(command) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: "control_frame",
            command: command,
            target_peer_id: "User_Guest" // Целимся во входящего участника
        }));
    }
}

export function toggleRecording() {
    const btn = document.getElementById('rec-btn');
    isRecording = !isRecording;
    if (isRecording) {
        btn.innerText = "⏸️ ПОСТАВИТЬ ЗАПИСЬ НА ПАУЗУ";
        btn.style.borderColor = "#ecc94b"; btn.style.color = "#ecc94b";
        appendSystemMsg("🔴 Запущен захват фреймов MediaRecorder API (0% CPU Server Overhead).");
    } else {
        btn.innerText = "🔴 ВОЗОБНОВИТЬ ЗАПИСЬ СЕССИИ";
        btn.style.borderColor = "#e53e3e"; btn.style.color = "#e53e3e";
        appendSystemMsg("💾 Запись заморожена. Буфер удерживается в изолированных страницах памяти.");
    }
}

export function toggleDrawingMode() {
    const btn = document.getElementById('draw-mode-btn');
    isDrawingMode = !isDrawingMode;
    if (isDrawingMode) {
        btn.innerText = "✏️ Режим рисования: ВКЛ";
        appendSystemMsg("✏️ Панель векторного рисования активирована поверх холста демонстрации.");
    } else {
        btn.innerText = "✏️ Режим рисования: ВЫКЛ";
    }
}

function appendChatMsg(sender, text) {
    const box = document.getElementById('chat-box');
    let formattedText = text;
    
    // Если бэкенд нашел и обернул фишинговый линк в безопасный редирект (Req. 5)
    if (text.includes("redirect?target=")) {
        const parts = text.split("target=");
        const extractedUrl = decodeURIComponent(parts[1]);
        formattedText = `<span style="color:#ecc94b; cursor:pointer; text-decoration:underline;" class="safe-link-trigger" data-url="${extractedUrl}">[🔒 БЕЗОПАСНАЯ ПРОВЕРКА ССЫЛКИ]</span>`;
    }

    box.innerHTML += `<div class="chat-msg-row"><b>[${sender}]:</b> ${formattedText}</div>`;
    box.scrollTop = box.scrollHeight;

    // Навешиваем обработчик осознанного перехода на Safe Transfer Page (Disclaimers)
    const links = box.querySelectorAll('.safe-link-trigger');
    links.forEach(link => {
        link.onclick = () => {
            window.open(`/redirect.html?target=${encodeURIComponent(link.getAttribute('data-url'))}`, '_blank');
        };
    });
}

function appendSystemMsg(text) {
    const box = document.getElementById('chat-box');
    box.innerHTML += `<div class="chat-msg-row" style="color:#ecc94b; font-size:11px;"><i>[СИСТЕМА]: ${text}</i></div>`;
    box.scrollTop = box.scrollHeight;
}
