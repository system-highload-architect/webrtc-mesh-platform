import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

/**
 * executeServerRecordControl управляет START / STOP триггерами NVMe записи на бэкенде Go
 */
export function executeServerRecordControl() {
    if (!SessionState.isModerator || !SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;

    const recBtn = document.getElementById('rec-btn');
    if (!recBtn) return;

    if (!SessionState.isRecording) {
        // Выстреливаем плоский b2b фрейм старта записи в сокет
        SessionState.ws.send(JSON.stringify({
            type: "control_frame",
            command: "START_RECORD"
        }));
        
        SessionState.isRecording = true;
        recBtn.innerText = "⏸️ Стоп";
        recBtn.style.borderColor = "#ecc94b";
        recBtn.style.color = "#ecc94b";
    } else {
        // Выстреливаем плоский b2b фрейм остановки записи в сокет
        SessionState.ws.send(JSON.stringify({
            type: "control_frame",
            command: "STOP_RECORD"
        }));

        SessionState.isRecording = false;
        recBtn.innerText = "🔴 Запись";
        recBtn.style.borderColor = '#ef4444';
        recBtn.style.color = '#ef4444';

        // Формируем персистентную ссылку скачивания WebM через API Gateway балансировщика (:8080)
        if (!SessionState.currentServerRecordID) {
            SessionState.currentServerRecordID = "backup_rec_" + Date.now();
        }
        const downloadUrl = `http://${window.location.host}/api/v1/records/download?id=${SessionState.currentServerRecordID}`;
        
        logChat(`[СЕРВЕР] Запись сохранена. Ссылка: <a href="${downloadUrl}" style="color:#3b82f6; font-weight:bold; text-decoration:underline;" target="_blank">⬇️ СКАЧАТЬ WEB M</a>`);
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
    
    // Вставляем кнопку записи первой в пульт управления
    bar.insertBefore(recBtn, bar.firstChild);
}
