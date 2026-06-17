import { SessionState } from '../session_context.js';
import { sendMsg } from '../chat/send_message.js';
import { checkT9, handleTabCompletion } from '../chat/t9_predict.js';
import { toggleAudio } from '../buttons/toggle_mic.js';
import { toggleVideo } from '../buttons/toggle_cam.js';
import { toggleScreenShare } from '../buttons/screen_share.js';
import { togglePauseRoomSignal } from '../buttons/pause_room.js';
import { executeRemoteMuteTargeted, executeRemoteKickTargeted } from '../buttons/mod_orchestrator.js';
import { injectRecordButton } from '../buttons/mod_recorder.js';
import { hangUp } from '../buttons/hangup.js';

// ИСПРАВЛЕНО (Абсолютные b2b-импорты модулей блоков): Подключаем новые изолированные JS-файлы
// FIXED: Swapped relative targets to secure absolute path resolution routines in Windows environments
import { executeGlobalAudioBlock } from '/static/js/buttons/mod_mute_all_audio.js';
import { executeGlobalVideoBlock } from '/static/js/buttons/mod_mute_all_video.js';

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
    const chatInput = document.getElementById('chat-input');
    const t9Placeholder = document.getElementById('t9-placeholder');
    const videoGrid = document.getElementById('video-grid');

    if (sendBtn) sendBtn.onclick = sendMsg;
    if (audioBtn) audioBtn.onclick = toggleAudio;
    if (videoBtn) videoBtn.onclick = toggleVideo;
    if (screenBtn) screenBtn.onclick = toggleScreenShare;
    
    if (hangupBtn) hangupBtn.onclick = hangUp;
    if (leaveBtn) leaveBtn.onclick = hangUp;

    if (chatInput && t9Placeholder) {
        let activeSuggestion = "";
        chatInput.oninput = async () => { activeSuggestion = await checkT9(chatInput, t9Placeholder); };
        chatInput.onkeydown = (e) => {
            if (e.key === "Tab" && activeSuggestion) {
                e.preventDefault();
                handleTabCompletion(chatInput, t9Placeholder, activeSuggestion);
                activeSuggestion = "";
            }
            if (e.key === "Enter") { sendMsg(); activeSuggestion = ""; }
        };
    }

    if (SessionState.isModerator && videoGrid) {
        videoGrid.onclick = (e) => {
            const muteTarget = e.target.closest('.target-mute-btn');
            const kickTarget = e.target.closest('.target-kick-btn');
            if (muteTarget) { e.stopPropagation(); executeRemoteMuteTargeted(muteTarget.getAttribute('data-peer')); }
            if (kickTarget) { e.stopPropagation(); executeRemoteKickTargeted(kickTarget.getAttribute('data-peer')); }
        };

        // Твоя оригинальная Пауза — сохранена без единого изменения
        const pauseBtn = document.getElementById('pause-btn-action');
        if (pauseBtn) pauseBtn.onclick = togglePauseRoomSignal;

        // ИСПРАВЛЕНО (Бинд кнопок тотального Блока звука и видео Давида):
        // Привязываем клики к новым инлайновым идентификаторам кнопок из conference.html
        const muteAllAudioBtn = document.getElementById('mute-all-audio-btn');
        if (muteAllAudioBtn) muteAllAudioBtn.onclick = executeGlobalAudioBlock;

        const muteAllVideoBtn = document.getElementById('mute-all-video-btn');
        if (muteAllVideoBtn) muteAllVideoBtn.onclick = executeGlobalVideoBlock;

        injectRecordButton();
    }
}
