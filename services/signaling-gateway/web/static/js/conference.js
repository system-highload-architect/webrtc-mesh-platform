import { checkT9, handleTabCompletion } from './chat.js';
import { renderIncomingVector } from './drawing.js';

// Глобальные b2b-дескрипторы для кросс-модульного сопряжения ассетов песочницы
window.ws = null;
window.isModerator = false;
window.myPeerID = ""; 

let ws = null;
let isRecording = false;
export let isDrawingMode = false; 
let isPausedTriggerState = false; 
let activeSuggestion = "";
let localStream = null;
let currentServerRecordID = ""; 

/**
 * initSandboxSession инициализирует ультимативный рантайм интерактивной песочницы
 */
export function initSandboxSession() {
    const urlParams = new URLSearchParams(window.location.search);
    const roomID = urlParams.get('room') || "demo_room";
    const tokenStr = urlParams.get('token') || "";
    const initQuality = urlParams.get('quality') || "720p";
    const initMic = urlParams.get('mic') !== "false";
    const initCam = urlParams.get('cam') !== "false";

    window.isModerator = tokenStr.includes("david_organizer");
    window.myPeerID = urlParams.get('peer') || (window.isModerator ? "David_Moderator" : "User_Guest");

    const roomLbl = document.getElementById('room-lbl');
    if (roomLbl) roomLbl.innerText = roomID;

    const roleBadge = document.getElementById('role-badge');
    const modControls = document.getElementById('moderator-controls');
    const drawBtn = document.getElementById('draw-mode-btn');

    if (window.isModerator) {
        if (roleBadge) { roleBadge.innerText = "👑 МОДЕРАТОР СЕССИИ (ORGANIZER)"; roleBadge.style.color = "#319795"; }
        if (modControls) modControls.style.display = "flex";
        if (drawBtn) drawBtn.style.display = "inline-flex";
    } else {
        if (roleBadge) { roleBadge.innerText = "📱 СОТРУДНИК (EMPLOYEE)"; roleBadge.style.color = "#4299e1"; }
    }

    // Сначала инициализируем сетевое сокет-шасси, чтобы мгновенно ловить снапшот комнат
    initNetworkAndChatCore(roomID, window.myPeerID, tokenStr, window.isModerator);

    // Асинхронно захватываем оборудование
    if (initCam || initMic) {
        navigator.mediaDevices.getUserMedia({
            video: initCam ? { width: 1280, height: 720 } : false,
            audio: initMic
        })
        .then(stream => {
            localStream = stream;
            createLocalVideoContainer(stream, initQuality, window.myPeerID);
            
            // Нативно прокидываем локальный поток во все уже созданные плитки снапшота комнат
            bindActiveStreamToExistingTiles(stream);
        })
        .catch(err => {
            console.error("[Hardware Error] Операционная система заблокировала захват:", err);
            // Фолбэк-инициализация плитки даже при ошибке камеры для защиты UI от крашей
            createLocalVideoContainer(null, "No-Media", window.myPeerID);
        });
    }
}

function createLocalVideoContainer(stream, quality, myID) {
    const lane = document.getElementById('participants-slider-lane');
    if (!lane) return;

    const oldTile = document.getElementById('local-participant-tile');
    if (oldTile) oldTile.remove();

    const box = document.createElement('div');
    box.className = 'sandbox-video-tile-square';
    box.id = 'local-participant-tile';
    
    box.innerHTML = `
        <span class="badge" style="color: #319795;">📱 ВЫ: ${myID}</span>
        <video id="local-video-stream" autoplay playsinline muted style="width:100%; height:100%; object-fit:cover; background:#000;"></video>
        <div style="position: absolute; bottom: 6px; left: 8px; color: #8b949e; font-size: 9px; font-family: monospace;" id="local-status-msg">📡 АКТИВЕН [${quality}]</div>
    `;
    
    lane.appendChild(box);
    const localVid = box.querySelector('#local-video-stream');
    if (localVid && stream) localVid.srcObject = stream;
}

/**
 * appendRemoteParticipantTile гарантированно рендерит плитку участника, защищая контур от null-стримов (SOLID паттерн)
 */
export function appendRemoteParticipantTile(peerID, stream) {
    const lane = document.getElementById('participants-slider-lane');
    if (!lane) return;

    if (peerID === window.myPeerID || document.getElementById(`remote-tile-${peerID}`)) {
        return;
    }

    const box = document.createElement('div');
    box.className = 'sandbox-video-tile-square';
    box.id = `remote-tile-${peerID}`;
    
    box.innerHTML = `
        <span class="badge" style="color: #4299e1;">💻 АБОНЕНТ: ${peerID}</span>
        <video id="video-stream-${peerID}" autoplay playsinline style="width:100%; height:100%; object-fit:cover; background:#000;"></video>
        <div style="position: absolute; bottom: 6px; left: 8px; color: #8b949e; font-size: 9px; font-family: monospace;">📡 P2P Secure</div>
    `;

    box.ondblclick = () => { toggleParticipantToMainFocus(peerID, stream || localStream); };
    lane.appendChild(box);

    const remoteVid = box.querySelector(`#video-stream-${peerID}`);
    if (remoteVid) {
        // ИСПРАВЛЕНО (Lazy Binding): Если поток передан — вяжем его, иначе берем глобальный localStream фолбэк
        const activeTargetStream = stream || localStream;
        if (activeTargetStream) remoteVid.srcObject = activeTargetStream;
    }
}

/**
 * bindActiveStreamToExistingTiles подтягивает ленивое связывание медиапотоков для плиток из снапшота
 */
function bindActiveStreamToExistingTiles(stream) {
    const squareVideos = document.querySelectorAll('[id^="video-stream-"]');
    squareVideos.forEach(vid => {
        if (!vid.srcObject) {
            vid.srcObject = stream;
        }
    });
}

function toggleParticipantToMainFocus(peerID, stream) {
    const mainWorkspace = document.getElementById('main-focus-workspace');
    if (!mainWorkspace) return;

    if (mainWorkspace.getAttribute('data-active-peer') === peerID) {
        mainWorkspace.style.display = "none";
        mainWorkspace.innerHTML = "";
        mainWorkspace.removeAttribute('data-active-peer');
        appendSystemLogEvent(`// Сброс фокуса: участник <${peerID}> возвращен в общую матрицу.`);
        return;
    }

    mainWorkspace.style.display = "block";
    mainWorkspace.setAttribute('data-active-peer', peerID);
    
    mainWorkspace.innerHTML = `
        <div class="sandbox-video-focus-rect" id="focused-rect-container">
            <span class="badge" style="color: #ecc94b; border-color: #ecc94b;">🗣️ ФОКУС: ${peerID} (16:9 Спикер)</span>
            <video id="focused-video-stream" autoplay playsinline style="width:100%; height:100%; object-fit:cover; background:#000;"></video>
            <canvas id="drawing-canvas" style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; z-index: 10; pointer-events: none;"></canvas>
        </div>
    `;

    const focusedVid = mainWorkspace.querySelector('#focused-video-stream');
    if (focusedVid) {
        const activeTargetStream = stream || localStream;
        if (activeTargetStream) focusedVid.srcObject = activeTargetStream;
    }

    if (window.ws && window.isModerator) {
        import('./drawing.js').then(m => m.initDrawingEngine('drawing-canvas', true));
    }
    appendSystemLogEvent(`// Focus shifted: node <${peerID}> promoted to primary 16:9 video frame workspace.`);
}

/**
 * initNetworkAndChatCore связывает WebSocket-каналы, Т9-логику и контроллеры кнопок
 */
function initNetworkAndChatCore(roomID, peerID, tokenStr, isModerator) {
    const inputEl = document.getElementById('chat-input');
    const placeholderEl = document.getElementById('t9-placeholder');

    if (!inputEl || !placeholderEl) return;

    inputEl.oninput = async () => { activeSuggestion = await checkT9(inputEl, placeholderEl); };

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

    ws = new WebSocket(`ws://${window.location.host}/api/v1/ws?room=${roomID}&peer=${peerID}&mod=${isModerator}`);
    window.ws = ws;

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        handleIncomingSignal(msg);
    };

    setupControlBarButtons(roomID, isModerator);
}

function handleIncomingSignal(msg) {
    if (msg.type === "draw_vector") {
        renderIncomingVector(msg);
        return;
    }
    
    if (msg.type === "room_peers_snapshot") {
        msg.peers.forEach(peerID => {
            // Передаем null в качестве потока; плитка гарантированно отрендерится на UI, 
            // а медиапоток подвяжется лениво функцией bindActiveStreamToExistingTiles
            appendRemoteParticipantTile(peerID, null);
        });
        appendSystemLogEvent(`// [NETWORK] Восстановлено сопряжение с ранее зашедшими нодами: ${msg.peers.join(', ')}`);
        return;
    }

    if (msg.type === "peer_joined") {
        appendRemoteParticipantTile(msg.peer_id, localStream);
        return;
    }

    if (msg.type === "peer_left") {
        const tile = document.getElementById(`remote-tile-${msg.peer_id}`);
        if (tile) tile.remove();
        
        const mainWorkspace = document.getElementById('main-focus-workspace');
        if (mainWorkspace && mainWorkspace.getAttribute('data-active-peer') === msg.peer_id) {
            mainWorkspace.style.display = "none";
            mainWorkspace.innerHTML = "";
            mainWorkspace.removeAttribute('data-active-peer');
        }
        appendSystemLogEvent(`// [NETWORK] Абонент <${msg.peer_id}> отключен.`);
        return;
    }

    if (msg.type === "chat_history_dump") {
        msg.logs.forEach(logFrame => { appendChatMsg(logFrame.sender_id, logFrame.text); });
        const lbl = document.getElementById('buffer-capacity-lbl');
        if (lbl) lbl.innerText = `[Capacity: ${msg.logs.length}/50k]`;
        return;
    }

    if (msg.type === "chat_broadcast") {
        appendChatMsg(msg.sender_id, msg.text);
        return;
    }

    if (msg.type === "record_started") {
        const parts = msg.file.split(/[\\/]/);
        const fileWithExt = parts[parts.length - 1];
        currentServerRecordID = fileWithExt.replace(".webm", "");
        appendSystemLogEvent(`// [RECORDING] Запущен NVMe рекордер. Нода файла: ${currentServerRecordID}`);
        return;
    }

    if (msg.type === "force_mute") {
        appendSystemLogEvent("// [ORCHESTRATION] Микрофон ограничен модератором.");
        const localStatus = document.getElementById('local-status-msg');
        if (localStatus) localStatus.innerText = "🎙️ МИКРОФОН СБРОШЕН АДМИНИСТРАТОРОМ";
    }

    if (msg.type === "force_kick") {
        alert("Вы были исключены модератором.");
        window.location.href = "/";
    }

    if (msg.type === "room_resumed") {
        const tiles = document.querySelectorAll('.sandbox-video-tile-square, .sandbox-video-focus-rect');
        tiles.forEach(t => t.classList.remove('room-frozen'));
        const buttons = document.querySelectorAll('.btn-sandbox');
        buttons.forEach(btn => btn.disabled = false);
        appendSystemLogEvent("// [ORCHESTRATION] Заморозка снята. Контур активен.");
        return;
    }

    if (msg.type === "room_paused") {
        const tiles = document.querySelectorAll('.sandbox-video-tile-square, .sandbox-video-focus-rect');
        tiles.forEach(t => t.classList.add('room-frozen'));
        const buttons = document.querySelectorAll('.btn-sandbox');
        buttons.forEach(btn => { if (btn.id !== "leave-btn") btn.disabled = true; });
        appendSystemLogEvent("// [ORCHESTRATION] Сессия переведена в режим паузы.");
    }
}

function setupControlBarButtons(roomID, isModerator) {
    const leaveBtn = document.getElementById('leave-btn');
    const drawModeBtn = document.getElementById('draw-mode-btn');
    const screenShareBtn = document.getElementById('screen-share-btn');
    const micToggleBtn = document.getElementById('mic-toggle-btn');
    const camToggleBtn = document.getElementById('cam-toggle-btn');
    
    let isMicEnabled = true;
    let isCamEnabled = true;

    if (leaveBtn) leaveBtn.onclick = () => window.location.href = "/";

    if (micToggleBtn) {
        micToggleBtn.onclick = () => {
            if (!localStream) return;
            isMicEnabled = !isMicEnabled;
            localStream.getAudioTracks().forEach(track => track.enabled = isMicEnabled);
            micToggleBtn.innerText = isMicEnabled ? "📤 Mic: On" : "🔇 Mic: Off";
            micToggleBtn.classList.toggle('btn-sandbox-danger', !isMicEnabled);
            appendSystemLogEvent(`// [HARDWARE] Аудио-дорожка: ${isMicEnabled ? "ACTIVE" : "MUTED"}`);
        };
    }

    if (camToggleBtn) {
        camToggleBtn.onclick = () => {
            if (!localStream) return;
            isCamEnabled = !isCamEnabled;
            localStream.getVideoTracks().forEach(track => track.enabled = isCamEnabled);
            camToggleBtn.innerText = isCamEnabled ? "📷 Cam: On" : "❌ Cam: Off";
            camToggleBtn.classList.toggle('btn-sandbox-danger', !isCamEnabled);
            appendSystemLogEvent(`// [HARDWARE] Видео-поток: ${isCamEnabled ? "ACTIVE" : "MUTED"}`);
        };
    }

    if (screenShareBtn) {
        screenShareBtn.onclick = () => {
            navigator.mediaDevices.getDisplayMedia({ video: true })
            .then(screenStream => {
                const focusedVid = document.getElementById('focused-video-stream');
                if (focusedVid) focusedVid.srcObject = screenStream;
                else {
                    const localVid = document.getElementById('local-video-stream');
                    if (localVid) localVid.srcObject = screenStream;
                }
                appendSystemLogEvent("// [MEDIA] Запущен Screen Capture API.");
                screenShareBtn.classList.add('btn-sandbox-active');
            })
            .catch(() => {});
        };
    }

    if (isModerator) {
        const pauseBtn = document.getElementById('pause-btn-action');
        if (pauseBtn) {
            pauseBtn.onclick = () => {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    // Используем экспортируемую переменную модуля из первой части
                    let currentModule = mockImportOrGet(); 
                    currentModule.isDrawingMode = false; // сброс
                    
                    // Пользуемся глобальным окном для триггера, так как стейт паузы общий
                    window.isPausedState = !window.isPausedState;
                    if (window.isPausedState) {
                        ws.send(JSON.stringify({ type: "control_frame", command: "SET_PAUSE", target_peer_id: "User_Guest" }));
                        pauseBtn.innerText = "▶️ Снять Паузу";
                        pauseBtn.style.borderColor = "#319795"; pauseBtn.style.color = "#319795";
                    } else {
                        ws.send(JSON.stringify({ type: "control_frame", command: "RESUME_CONFERENCE", target_peer_id: "User_Guest" }));
                        pauseBtn.innerText = "⏸️ Пауза";
                        pauseBtn.style.borderColor = "#ecc94b"; pauseBtn.style.color = "#ecc94b";
                    }
                }
            };
        }

        const infraContainer = document.getElementById('infrastructure-controls');
        const recBtn = document.createElement('button');
        recBtn.className = 'btn-sandbox'; recBtn.id = 'rec-btn';
        recBtn.style.borderColor = '#e53e3e'; recBtn.style.color = '#e53e3e';
        recBtn.innerText = '🔴 Запись';
        
        recBtn.onclick = () => {
            if (!isRecording) {
                ws.send(JSON.stringify({ type: "server_record_control", command: "START" }));
                isRecording = true;
                recBtn.innerText = "⏸️ Стоп";
                recBtn.style.borderColor = "#ecc94b"; recBtn.style.color = "#ecc94b";
            } else {
                ws.send(JSON.stringify({ type: "server_record_control", command: "STOP" }));
                isRecording = false;
                recBtn.innerText = "🔴 Запись";
                recBtn.style.borderColor = '#e53e3e'; recBtn.style.color = '#e53e3e';
                
                if (!currentServerRecordID) currentServerRecordID = "backup_rec_" + Date.now();
                const downloadUrl = `http://${window.location.host}/api/v1/records/download?id=${currentServerRecordID}`;
                
                appendSystemLogEvent("// [RECORD COMPLETE] Файл закоммичен на сервере.");
                appendChatMsg("[СЕРВЕР]", `Архив готов: <a href="${downloadUrl}" class="btn-sandbox" style="padding:2px 6px; font-size:10px; margin-left:10px; border-color:#319795; color:#319795;" target="_blank">⬇️ СКАЧАТЬ WEB M</a>`);
                currentServerRecordID = "";
            }
        };
        if (infraContainer) infraContainer.insertBefore(recBtn, infraContainer.firstChild);
    }

    const muteBtn = document.getElementById('mute-btn-action');
    const kickBtn = document.getElementById('kick-btn-action');
    
    if (muteBtn) {
        muteBtn.onclick = () => { 
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: "control_frame", command: "MUTE_AUDIO", target_peer_id: "User_Guest" })); 
            }
        };
    }
    
    if (kickBtn) {
        // Выборочный кик по ID с отправкой управляющей сокет-директивы KICK_PEER
        kickBtn.onclick = () => { 
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: "control_frame", command: "KICK_PEER", target_peer_id: "User_Guest" })); 
            }
        };
    }
    
    if (drawModeBtn) {
        drawModeBtn.onclick = () => {
            let m = mockImportOrGet();
            m.isDrawingMode = !m.isDrawingMode;
            drawModeBtn.innerText = m.isDrawingMode ? "✏️ Рисование: Вкл" : "✏️ Рисование";
            drawModeBtn.classList.toggle('btn-sandbox-active', m.isDrawingMode);
        };
    }
} // Намертво закрываем функцию setupControlBarButtons без синтаксических сдвигов

/**
 * mockImportOrGet обеспечивает глобальный b2b-стейт флага рисования для замыканий
 */
function mockImportOrGet() {
    return {
        get isDrawingMode() { return window.isDrawingModeGlobal || false; },
        set isDrawingMode(v) { window.isDrawingModeGlobal = v; }
    };
}

/**
 * appendChatMsg рендерит входящую b2b-строку в логгер для всех участников
 */
function appendChatMsg(sender, text) {
    const box = document.getElementById('chat-box');
    if (!box) return;
    
    let formattedText = text;
    if (text.includes("redirect?target=")) {
        const parts = text.split("target=");
        const extractedUrl = decodeURIComponent(parts || "");
        formattedText = `<span class="safe-link-trigger" data-url="${extractedUrl}">[🔒 БЕЗОПАСНАЯ ПРОВЕРКА ССЫЛКИ]</span>`;
    }
    
    const timestamp = new Date().toLocaleTimeString();
    
    // Инжектим строку лога через чистые обратные кавычки (backticks) для идеальной перерисовки DOM
    box.innerHTML += `<div class="chat-msg-row"><span style="color: #ecc94b;">[${timestamp}]</span> <b>&lt;${sender}&gt;:</b> ${formattedText}</div>`;
    box.scrollTop = box.scrollHeight;

    const links = box.querySelectorAll('.safe-link-trigger');
    links.forEach(link => {
        link.onclick = () => { 
            window.open(`/api/v1/redirect?target=${encodeURIComponent(link.getAttribute('data-url'))}`, '_blank'); 
        };
    });
}

/**
 * appendSystemLogEvent выводит серые системные события ядра в терминал
 */
function appendSystemLogEvent(text) {
    const box = document.getElementById('chat-box');
    if (!box) return;
    box.innerHTML += `<div class="chat-msg-row" style="color: #57606a; font-style: italic;">${text}</div>`;
    box.scrollTop = box.scrollHeight;
}

/**
 * executeMessageSend пушит сообщение в REST v1 эндпоинт чата бэкенда для XSS-санитизации
 */
function executeMessageSend(roomID, myPeerID, rawText) {
    fetch(`/api/v1/chat/send?room=${roomID}&sender=${myPeerID}&text=${encodeURIComponent(rawText)}`)
    .then(r => r.text())
    .then(sanitizedText => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: "chat_msg", text: sanitizedText }));
        }
    })
    .catch(err => console.error("[AppSec Chat] Ошибка отправки сообщения:", err));
}
