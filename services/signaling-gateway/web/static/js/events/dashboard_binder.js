import { sendMsg } from '../chat/send_message.js';
import { toggleAudio } from '../buttons/toggle_mic.js';
import { toggleVideo } from '../buttons/toggle_cam.js';
import { toggleScreenShare } from '../buttons/screen_share.js';
import { hangUp } from '../buttons/hangup.js';

/**
 * bindDashboardEvents привязывает изолированные ESM-обработчики к кнопкам управления UI
 */
export function bindDashboardEvents() {
    const sendBtn = document.getElementById('chat-send-btn');
    const audioBtn = document.getElementById('audio-toggle');
    const videoBtn = document.getElementById('video-toggle');
    const screenBtn = document.getElementById('screen-toggle');
    const hangupBtn = document.getElementById('hangup-btn');
    const leaveBtn = document.getElementById('leave-btn');

    // Навешиваем атомарные бизнес-функции на клики кнопок нижнего и бокового пультов
    if (sendBtn) sendBtn.onclick = sendMsg;
    if (audioBtn) audioBtn.onclick = toggleAudio;
    if (videoBtn) videoBtn.onclick = toggleVideo;
    if (screenBtn) screenBtn.onclick = toggleScreenShare;
    
    // Кнопка «Завершить» на панели и «Покинуть контур» на статус-баре вызывают один деструктор
    if (hangupBtn) hangupBtn.onclick = hangUp;
    if (leaveBtn) leaveBtn.onclick = hangUp;
}
