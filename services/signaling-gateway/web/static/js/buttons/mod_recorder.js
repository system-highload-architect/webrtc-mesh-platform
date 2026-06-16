import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

let internalMediaRecorder = null;

/**
 * executeServerRecordControl управляет START / STOP триггерами записи на бэкенде Go
 */
export function executeServerRecordControl() {
    if (!SessionState.isModerator || !SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;

    const recBtn = document.getElementById('rec-btn');
    const localVideo = document.getElementById('local-video');
    if (!recBtn || !localVideo) return;

    if (!SessionState.isRecording) {
        SessionState.currentServerRecordID = "";

        // 1. Посылаем плоскую команду инициализации дескриптора на Бэкенд
        SessionState.ws.send(JSON.stringify({
            type: "control_frame",
            command: "START_RECORD"
        }));
        
        // 2. Аппаратно перехватываем единый, непрерывный видеопоток прямо с HTML-окна
        try {
            // Нативно захватываем объединенный поток (веб-камера или демка экрана — пишется всё, что видит глаз)
            const captureStream = localVideo.captureStream ? localVideo.captureStream(30) : localVideo.mozCaptureStream(30);
            
            // Подмешиваем аудио-дорожку локального микрофона модератора для записи голоса
            if (SessionState.localStream && SessionState.localStream.getAudioTracks().length > 0) {
                SessionState.localStream.getAudioTracks().forEach(track => {
                    captureStream.addTrack(track);
                });
            }

            // Инициализируем нативный MediaRecorder браузера со стандартным кодеком VP8/Opus
            internalMediaRecorder = new MediaRecorder(captureStream, { mimeType: 'video/webm;codecs=vp8,opus' });

            // Нарезаем кадры в ArrayBuffer и шлем в сокет-шину бэка
            internalMediaRecorder.ondataavailable = async (event) => {
                if (event.data && event.data.size > 0 && SessionState.currentServerRecordID) {
                    const arrayBuffer = await event.data.arrayBuffer();
                    const uint8Array = new Uint8Array(arrayBuffer);
                    
                    if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                        SessionState.ws.send(JSON.stringify({
                            type: "record_chunk",
                            record_id: SessionState.currentServerRecordID,
                            media_bytes: Array.from(uint8Array)
                        }));
                    }
                }
            };

            // Нарезаем поток на b2b временные отрезки по 2000 миллисекунд
            internalMediaRecorder.start(2000);
            
            SessionState.isRecording = true;
            recBtn.innerText = "⏸️ Стоп";
            recBtn.style.borderColor = "#ecc94b";
            recBtn.style.color = "#ecc94b";
        } catch (err) {
            console.error("[HARDWARE RECORDER] Не удалось инициализировать MediaRecorder кадра:", err.message);
            logChat(`// [REC ERROR] Крах инициализации кодека записи: ${err.message}`, "#ef4444");
        }
    } else {
        // ОСТАНОВКА СЕССИИ ЗАПИСИ
        if (internalMediaRecorder && internalMediaRecorder.state !== "inactive") {
            internalMediaRecorder.stop();
        }

        SessionState.ws.send(JSON.stringify({
            type: "control_frame",
            command: "STOP_RECORD"
        }));

        SessionState.isRecording = false;
        recBtn.innerText = "🔴 Запись";
        recBtn.style.borderColor = '#ef4444';
        recBtn.style.color = '#ef4444';

        const fileId = SessionState.currentServerRecordID || ("backup_rec_" + Date.now());
        const downloadUrl = `http://${window.location.host}/api/v1/records/download?id=${fileId}`;
        
        logChat(`[СЕРВЕР] NVMe gRPC-запись запечатана. Ссылка: <a href="${downloadUrl}" style="color:#3b82f6; font-weight:bold; text-decoration:underline;" target="_blank">⬇️ СКАЧАТЬ ВАЛИДНЫЙ WEB M</a>`);
    }
}

/**
 * injectRecordButton динамически встраивает кнопку записи в дашборд модератора Давида
 */
export function injectRecordButton() {
    const bar = document.getElementById('infrastructure-controls');
    if (!bar || document.getElementById('rec-btn')) return;

    const recBtn = document.createElement('button');
    recBtn.className = 'ctrl-btn';
    recBtn.id = 'rec-btn';
    recBtn.style.cssText = "background-color:#1e293b; border:1px solid #ef4444; color:#ef4444; padding:10px 16px; border-radius:6px; cursor:pointer; font-size:0.75rem; font-family:monospace; margin-right:8px;";
    recBtn.innerText = '🔴 Запись';
    
    recBtn.onclick = executeServerRecordControl;
    bar.insertBefore(recBtn, bar.firstChild);
}

// Привязываем перехват прилетающего ID записи от сокет-менеджера в рантайме сессии
window.setServerRecordSessionID = (fileId) => {
    SessionState.currentServerRecordID = fileId;
};
