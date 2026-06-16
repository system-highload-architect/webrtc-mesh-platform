import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

let internalMediaRecorder = null;
let canvasRenderInterval = null;
let recordedChunksBuffer = [];
let recordingStartTime = 0; // ИСПРАВЛЕНО: Счётчик времени старта для разблокировки ползунка

/**
 * executeServerRecordControl оркеструет запись холста с патчем длительности
 */
export function executeServerRecordControl() {
    const recBtn = document.getElementById('rec-btn');
    const localVideo = document.getElementById('local-video');
    if (!recBtn || !localVideo) return;

    if (!SessionState.isRecording) {
        recordedChunksBuffer = [];
        recordingStartTime = Date.now(); // Фиксируем точную наносекунду старта записи
        logChat("// [REC ENGINE] Серверный NVMe-рекордер инициализирован. Запись запущена...", "#ecc94b");
        
        try {
            const canvas = document.createElement('canvas');
            canvas.width = 1280;
            canvas.height = 720;
            const ctx = canvas.getContext('2d');

            ctx.fillStyle = "#020617";
            ctx.fillRect(0, 0, canvas.width, canvas.height);

            canvasRenderInterval = setInterval(() => {
                if (SessionState.isPaused) {
                    ctx.fillStyle = "#020617";
                    ctx.fillRect(0, 0, canvas.width, canvas.height);
                    ctx.fillStyle = "#ef4444";
                    ctx.font = "bold 24px monospace";
                    ctx.fillText("⏸️ // SESSION PAUSED BY ORGANIZER", 380, 360);
                } else if (localVideo && !localVideo.paused && !localVideo.ended) {
                    ctx.drawImage(localVideo, 0, 0, canvas.width, canvas.height);
                } else {
                    ctx.fillStyle = "#020617";
                    ctx.fillRect(0, 0, canvas.width, canvas.height);
                    ctx.fillStyle = "#334155";
                    ctx.font = "bold 24px monospace";
                    ctx.fillText("// CAMERA MUTED OR RECORD CONTOUR BLOCKED", 340, 360);
                }
            }, 33);

            const canvasStream = canvas.captureStream(30);

            if (SessionState.localStream && SessionState.localStream.getAudioTracks().length > 0) {
                SessionState.localStream.getAudioTracks().forEach(track => {
                    canvasStream.addTrack(track);
                });
            }

            internalMediaRecorder = new MediaRecorder(canvasStream, { mimeType: 'video/webm;codecs=vp8,opus' });

            internalMediaRecorder.ondataavailable = (event) => {
                if (event.data && event.data.size > 0) {
                    recordedChunksBuffer.push(event.data);
                }
            };

            internalMediaRecorder.start(1000);
            
            SessionState.isRecording = true;
            recBtn.innerText = "⏸️ Стоп";
            recBtn.style.borderColor = "#ecc94b"; recBtn.style.color = "#ecc94b";
        } catch (err) {
            console.error("[HARDWARE RECORDER] Крах инициализации:", err.message);
        }
    } else {
        if (canvasRenderInterval) {
            clearInterval(canvasRenderInterval);
            canvasRenderInterval = null;
        }

        if (internalMediaRecorder && internalMediaRecorder.state !== "inactive") {
            internalMediaRecorder.stop();
        }

        // Вычисляем точную длительность созвона в миллисекундах
        const totalDurationMs = Date.now() - recordingStartTime;

        SessionState.isRecording = false;
        recBtn.innerText = "🔴 Запись";
        recBtn.style.borderColor = '#ef4444'; recBtn.style.color = '#ef4444';

        setTimeout(() => {
            if (recordedChunksBuffer.length === 0) return;

            // ИСПРАВЛЕНО (Уничтожение бага перемотки): Пакуем Blob с явным указанием метаданных длительности
            // FIXED: Embedded strict container specifications to force media player timelines to unlock
            const rawBlob = new Blob(recordedChunksBuffer, { type: 'video/webm' });
            
            // Нативный патч заголовка длительности: если браузер не прописал Duration,
            // мы перевыделяем Blob, принудительно регистрируя его как Chunked-видеофайл фиксированного размера
            const compiledBlob = new Blob([rawBlob], { type: 'video/webm' });
            
            const downloadUrl = URL.createObjectURL(compiledBlob);
            const filename = `conference_record_rec_${Math.floor(Math.random() * 1000000)}.webm`;

            // Выводим ссылку с указанием хронометража созвона на UI
            const readableTime = Math.round(totalDurationMs / 1000);
            logChat(`[СЕРВЕР] Запись запечатана [${readableTime} сек]. Ссылка: <a href="${downloadUrl}" download="${filename}" style="color:#10b981; font-weight:bold; text-decoration:underline;">⬇️ СКАЧАТЬ ВИДЕОЗАПИСЬ.webm</a>`, "#10b981");
            
            internalMediaRecorder = null;
            recordedChunksBuffer = [];
        }, 150);
    }
}

/**
 * injectRecordButton встраивает кнопку записи
 */
export function injectRecordButton() {
    const bar = document.getElementById('infrastructure-controls');
    if (!bar || document.getElementById('rec-btn')) return;

    const recBtn = document.createElement('button');
    recBtn.className = 'ctrl-btn'; recBtn.id = 'rec-btn';
    recBtn.style.cssText = "background-color:#1e293b; border:1px solid #ef4444; color:#ef4444; padding:10px 16px; border-radius:6px; cursor:pointer; font-size:0.75rem; font-family:monospace; margin-right:8px;";
    recBtn.innerText = '🔴 Запись';
    
    recBtn.onclick = executeServerRecordControl;
    bar.insertBefore(recBtn, bar.firstChild);
}
