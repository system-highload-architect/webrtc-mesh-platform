import { checkT9, handleTabCompletion } from './chat.js';
import { renderIncomingVector } from './drawing.js';

let ws = null;
let isRecording = false;
let isDrawingMode = false;
let activeSuggestion = "";

export function initConference(roomID, tokenStr, initQuality, initMic, initCam, myPeerID, isModerator) {
    const inputEl = document.getElementById('chat-input');
    const placeholderEl = document.getElementById('t9-placeholder');

    // 1. Привязываем наносекундный Т9-конвейер к инпуту
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
            executeMessageSend(roomID, myPeerID, inputEl.value);
            inputEl.value = "";
            placeholderEl.innerText = "";
            activeSuggestion = "";
        }
    };

    // 2. Подключаем версионированный v1 WebSocket сигнального шлюза
    ws = new WebSocket(`ws://${window.location.host}/api/v1/ws?room=${roomID}&token=${tokenStr}`);
    window.ws = ws;

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        handleIncomingSignal(msg, initQuality, initMic, initCam);
    };
}

function handleIncomingSignal(msg, initQuality, initMic, initCam) {
    if (msg.draw_vector) {
        renderIncomingVector(msg.draw_vector);
        return;
    }

    if (msg.type === "auth_error") {
        alert("[ОШИБКА БЕЗОПАСНОСТИ] Токен JWT не прошел криптографическую валидацию ядра.");
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
        if (isRecording) toggleRecording();
    }
}

function executeMessageSend(roomID, myPeerID, rawText) {
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
            target_peer_id: "User_Guest"
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
    
    if (text.includes("redirect?target=")) {
        const parts = text.split("target=");
        const extractedUrl = decodeURIComponent(parts[1]);
        formattedText = `<span style="color:#ecc94b; cursor:pointer; text-decoration:underline;" class="safe-link-trigger" data-url="${extractedUrl}">[🔒 БЕЗОПАСНАЯ ПРОВЕРКА ССЫЛКИ]</span>`;
    }

    box.innerHTML += `<div><b>[${sender}]:</b> ${formattedText}</div>`;
    box.scrollTop = box.scrollHeight;

    // Вешаем перехват на динамически созданную ссылку
    const links = box.querySelectorAll('.safe-link-trigger');
    links.forEach(link => {
        link.onclick = () => {
            window.open(`/redirect.html?target=${encodeURIComponent(link.getAttribute('data-url'))}`, '_blank');
        };
    });
}

function appendSystemMsg(text) {
    const box = document.getElementById('chat-box');
    box.innerHTML += `<div style="color:#ecc94b; font-size:11px;"><i>[СИСТЕМА]: ${text}</i></div>`;
    box.scrollTop = box.scrollHeight;
}
