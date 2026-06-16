import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';

/**
 * togglePauseRoomSignal инвертирует стейт паузы комнаты и пушит директиву в сокет
 */
export function togglePauseRoomSignal() {
    if (!SessionState.ws || SessionState.ws.readyState !== WebSocket.OPEN) return;
    
    SessionState.isPaused = !SessionState.isPaused;
    const pauseBtn = document.getElementById('pause-btn-action');
    
    if (pauseBtn) {
        if (SessionState.isPaused) {
            // Выстреливаем управляющий фрейм заморозки контура
            SessionState.ws.send(JSON.stringify({ 
                type: "control_frame", 
                payload: btoa(JSON.stringify({ command: "SET_PAUSE" })) 
            }));
            pauseBtn.innerText = "▶️ Снять Паузу";
            pauseBtn.style.borderColor = "#319795"; 
            pauseBtn.style.color = "#319795";
        } else {
            // Выстреливаем управляющий фрейм возобновления активного Mesh
            SessionState.ws.send(JSON.stringify({ 
                type: "control_frame", 
                payload: btoa(JSON.stringify({ command: "RESUME_CONFERENCE" })) 
            }));
            pauseBtn.innerText = "⏸️ Пауза";
            pauseBtn.style.borderColor = "#ecc94b"; 
            pauseBtn.style.color = "#ecc94b";
        }
    }
}

/**
 * toggleUiFreeze накладывает или снимает размытие и блэкаут-эффект со всех видеоокон
 */
export function toggleUiFreeze(freeze) {
    const wrappers = document.querySelectorAll('.video-wrapper');
    wrappers.forEach(w => w.classList.toggle('room-frozen', freeze));
    
    // Блокируем или оживляем интерактивные кнопки нижнего пульта сотрудников
    const buttons = document.querySelectorAll('.ctrl-btn, .btn-sandbox');
    buttons.forEach(btn => { 
        if (btn.id !== "hangup-btn" && btn.id !== "leave-btn" && btn.id !== "pause-btn-action") {
            btn.disabled = freeze; 
        }
    });

    logChat(freeze ? "// [ORCHESTRATION] Сессия переведена модератором в режим Паузы." : "// [ORCHESTRATION] Заморозка снята. Контур вещания активен.", "#ecc94b");
}
