import { SessionState } from '../session_context.js';
import { logChat } from '../chat/render_log.js';
import { toggleFullscreenElement } from './fullscreen.js';

/**
 * toggleScreenShare запускает захват экрана или останавливает текущую демонстрацию
 */
export async function toggleScreenShare() {
    const btn = document.getElementById('screen-toggle');
    if (!SessionState.isScreenSharing) {
        try {
            SessionState.screenStream = await navigator.mediaDevices.getDisplayMedia({ video: true });
            const screenTrack = SessionState.screenStream.getVideoTracks()[0];
            
            // 1. Подменяем дорожку во всех P2P-плечах Full-Mesh сети
            for (let peerId in SessionState.peerConnections) {
                const senders = SessionState.peerConnections[peerId].getSenders();
                const videoSender = senders.find(s => s.track && s.track.kind === 'video');
                if (videoSender) {
                    videoSender.replaceTrack(screenTrack);
                }
            }
            
            // ИСПРАВЛЕНО (Запись демонстрации экрана): Если Давид ведет запись, на лету инжектируем трек экрана в рекордер!
            // FIXED: Dynamically injected screen track into active recorder stream pipeline to fix blinding bugs
            if (SessionState.isRecording && window.internalMediaRecorderRef) {
                const mixedStream = window.internalMediaRecorderRef.stream;
                const activeVideoTrack = mixedStream.getVideoTracks()[0];
                if (activeVideoTrack) mixedStream.removeTrack(activeVideoTrack);
                mixedStream.addTrack(screenTrack);
            }
            
            const localVideo = document.getElementById('local-video');
            if (localVideo) {
                localVideo.srcObject = SessionState.screenStream;
                localVideo.style.transform = "none";
            }
            
            SessionState.isScreenSharing = true;
            if (btn) {
                btn.innerText = "🖥️ Экран: On";
                btn.style.backgroundColor = "#059669";
                btn.style.borderColor = "#10b981";
            }
            
            const localContainer = document.getElementById('local-video-container');
            if (localContainer) toggleFullscreenElement(localContainer);
            
            screenTrack.onended = () => stopScreenShare();
            logChat(`// [MEDIA] Демонстрация экрана успешно сопряжена с Mesh-нодами.`, "#10b981");
        } catch (e) {
            logChat(`// [MEDIA] Абонент отменил захват экрана: ${e.name}`, "#8b949e");
        }
    } else {
        stopScreenShare();
    }
}

/**
 * stopScreenShare корректно гасит стрим захвата экрана и возвращает поток с веб-камеры
 */
export function stopScreenShare() {
    if (!SessionState.isScreenSharing) return;
    
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
    
    if (SessionState.screenStream) {
        SessionState.screenStream.getTracks().forEach(track => track.stop());
        SessionState.screenStream = null;
    }
    
    const cameraTracks = SessionState.localStream ? SessionState.localStream.getVideoTracks() : [];
    if (cameraTracks.length > 0) {
        const cameraTrack = cameraTracks[0];
        
        for (let peerId in SessionState.peerConnections) {
            const senders = SessionState.peerConnections[peerId].getSenders();
            const videoSender = senders.find(s => s.track && s.track.kind === 'video');
            if (videoSender) {
                videoSender.replaceTrack(cameraTrack).catch(err => console.error(err));
            }
        }

        // ИСПРАВЛЕНО (Возврат камеры в запись): Принудительно возвращаем трек веб-камеры в рекордер
        if (SessionState.isRecording && window.internalMediaRecorderRef) {
            const mixedStream = window.internalMediaRecorderRef.stream;
            const activeVideoTrack = mixedStream.getVideoTracks()[0];
            if (activeVideoTrack) mixedStream.removeTrack(activeVideoTrack);
            mixedStream.addTrack(cameraTrack);
        }
    }
    
    const localVideo = document.getElementById('local-video');
    if (localVideo) {
        localVideo.srcObject = SessionState.localStream;
        localVideo.style.transform = "scaleX(-1)";
    }
    
    SessionState.isScreenSharing = false;
    const btn = document.getElementById('screen-toggle');
    if (btn) {
        btn.innerText = "🖥️ Демонстрация";
        btn.style.backgroundColor = "#1e293b";
        btn.style.borderColor = "#334155";
    }
    logChat(`// [MEDIA] Демонстрация экрана остановлена. Поток веб-камеры восстановлен.`, "#8b949e");
}
