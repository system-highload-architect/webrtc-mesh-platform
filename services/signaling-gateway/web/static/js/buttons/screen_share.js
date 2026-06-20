import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';
import { toggleFullscreenElement } from './fullscreen.js';
import { captureLocalMedia } from '../services/media_capture.js'; // Импортируем чистый перезапуск треков

/**
 * toggleScreenShare запускает захват экрана или останавливает текущую демонстрацию
 */
export async function toggleScreenShare() {
    const btn = document.getElementById('screen-toggle');
    if (!SessionState.isScreenSharing) {
        try {
            SessionState.screenStream = await navigator.mediaDevices.getDisplayMedia({ video: true });
            const screenTrack = SessionState.screenStream.getVideoTracks()[0];
            
            // Бесшовно подменяем дорожку во всех P2P-плечах Full-Mesh сети
            for (let peerId in SessionState.peerConnections) {
                const senders = SessionState.peerConnections[peerId].getSenders();
                const videoSender = senders.find(s => s.track && s.track.kind === 'video');
                if (videoSender) {
                    videoSender.replaceTrack(screenTrack);
                }
            }
            
            const localVideo = document.getElementById('local-video');
            if (localVideo) {
                localVideo.srcObject = SessionState.screenStream;
                localVideo.style.transform = "none"; // Отменяем зеркалирование для читаемости презентации
            }
            
            SessionState.isScreenSharing = true;
            if (btn) {
                btn.innerText = "🖥️ Экран: On";
                btn.style.backgroundColor = "#059669";
                btn.style.borderColor = "#10b981";
            }
            
            const localContainer = document.getElementById('local-video-container');
            if (localContainer) toggleFullscreenElement(localContainer);
            
            // Если сотрудник нажал системную кнопку браузера "Остановить совместный доступ"
            screenTrack.onended = () => stopScreenShare();
            logChat(`// [MEDIA] Демонстрация экрана успешно выведена в Mesh-контур.`, "#10b981");
        } catch (e) {
            logChat(`// [MEDIA] Захват экрана отклонен пользователем: ${e.name}`, "#8b949e");
        }
    } else {
        stopScreenShare();
    }
}

/**
 * stopScreenShare корректно гасит стрим захвата экрана и реанимирует веб-камеру
 */
export async function stopScreenShare() {
    if (!SessionState.isScreenSharing) return;
    
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
    
    // Аппаратно тушим треки экрана
    if (SessionState.screenStream) {
        SessionState.screenStream.getTracks().forEach(track => track.stop());
        SessionState.screenStream = null;
    }
    
    SessionState.isScreenSharing = false;
    const btn = document.getElementById('screen-toggle');
    if (btn) {
        btn.innerText = "🖥️ Демонстрация";
        btn.style.backgroundColor = "#1e293b";
        btn.style.borderColor = "#334155";
    }

    logChat(`// [MEDIA] Демонстрация остановлена. Перезапуск аппаратных треков веб-камеры...`, "#8b949e");
    
    // Нативно, с чистого листа инициализируем камеру!
    // Invoked clean media capture factory reset lifecycle to explicitly eliminate blank video viewport locks
    await captureLocalMedia();

    // Бесшовно возвращаем свежий, живой трек камеры во все активные P2P-плечи Mesh-кластера
    if (SessionState.localStream) {
        const freshCameraTrack = SessionState.localStream.getVideoTracks()[0];
        for (let peerId in SessionState.peerConnections) {
            const senders = SessionState.peerConnections[peerId].getSenders();
            const videoSender = senders.find(s => s.track && s.track.kind === 'video');
            if (videoSender && freshCameraTrack) {
                videoSender.replaceTrack(freshCameraTrack).catch(err => console.error(err));
            }
        }
    }
}
