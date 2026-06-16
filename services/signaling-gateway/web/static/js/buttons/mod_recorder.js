import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

let internalMediaRecorder = null;
let canvasRenderInterval = null;
let recordedChunksBuffer = [];

/**
 * executeServerRecordControl оркеструет пуленепробиваемую REST-выгрузку идеального WebM
 */
export function executeServerRecordControl() {
    const recBtn = document.getElementById('rec-btn');
    const localVideo = document.getElementById('local-video');
    if (!recBtn || !localVideo) return;

    if (!SessionState.isRecording) {
        recordedChunksBuffer = [];

        // Генерируем ID записи локально, чтобы сразу привязать его к REST-каналу выгрузки
        SessionState.currentServerRecordID = "rec_" + Date.now();

        logChat("// [REC ENGINE] Локальный NVMe-рекордер инициализирован. Запись запущена...", "#ecc94b");
        
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

            // Нативно и без нагрузки на CPU копим чистые Blob-кусочки видео во внутреннюю RAM-память вкладки
            internalMediaRecorder.ondataavailable = (event) => {
                if (event.data && event.data.size > 0) {
                    recordedChunksBuffer.push(event.data);
                }
            };

            // Нарезаем поток в буфер каждую секунду для максимальной отказоустойчивости
            internalMediaRecorder.start(1000);
            
            SessionState.isRecording = true;
            recBtn.innerText = "⏸️ Стоп";
            recBtn.style.borderColor = "#ecc94b"; recBtn.style.color = "#ecc94b";
        } catch (err) {
            console.error("[HARDWARE RECORDER] Крах инициализации:", err.message);
        }
    } else {
        // ОСТАНОВКА СЕССИИ ЗАПИСИ
        if (canvasRenderInterval) {
            clearInterval(canvasRenderInterval);
            canvasRenderInterval = null;
        }

        if (internalMediaRecorder && internalMediaRecorder.state !== "inactive") {
            internalMediaRecorder.stop();
        }

        SessionState.isRecording = false;
        recBtn.innerText = "🔴 Запись";
        recBtn.style.borderColor = '#ef4444'; recBtn.style.color = '#ef4444';

        // Выдерживаем микро-паузу в 200мс, чтобы кодек Chromium успел сбросить финальный Blob-кусок в массив
        setTimeout(() => {
            if (recordedChunksBuffer.length === 0) return;

            // Склеиваем накопленный буфер в один монолитный, идеально запечатанный WebM-файл
            const compiledBlob = new Blob(recordedChunksBuffer, { type: 'video/webm' });
            
            const fileId = SessionState.currentServerRecordID;
            const uploadUrl = `/api/v1/records/upload?id=${fileId}`;

            logChat("// [REC ENGINE] Верификация видеоконтейнера. Выгрузка WebM на NVMe-диск сервера...", "#ecc94b");

            // ИСПРАВЛЕНО (Ультимативная b2b доставка): Асинхронно выстреливаем готовый бинарный монолит на бэкенд прокси!
            // FIXED: Transmitted fully-compiled single binary blob package via optimized REST POST pipeline
            fetch(uploadUrl, {
                method: "POST",
                body: compiledBlob
            })
            .then(response => {
                if (response.ok) {
                    const downloadUrl = `http://${window.location.host}/api/v1/records/download?id=${fileId}`;
                    logChat(`[СЕРВЕР] Запись успешно сохранена на NVMe. Ссылка: <a href="${downloadUrl}" style="color:#10b981; font-weight:bold; text-decoration:underline;" target="_blank">⬇️ СКАЧАТЬ ВИДЕОЗАПИСЬ.webm</a>`, "#10b981");
                } else {
                    logChat("// [REC ERROR] Ошибка сохранения файла на стороне API Gateway.", "#ef4444");
                }
            })
            .catch(err => {
                console.error("[REC REST UPLOAD ERROR]:", err);
            });
            
            internalMediaRecorder = null;
            recordedChunksBuffer = [];
        }, 300);
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
