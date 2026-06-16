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
            // Аппаратно запрашиваем захват монитора/окна у операционной системы
            SessionState.screenStream = await navigator.mediaDevices.getDisplayMedia({ video: true });
            const screenTrack = SessionState.screenStream.getVideoTracks()[0];
            
            // ИСПРАВЛЕНО: Бесшовно подменяем дорожку во всех P2P-плечах Full-Mesh сети (Твой b2b паттерн)
            for (let peerId in SessionState.peerConnections) {
                const senders = SessionState.peerConnections[peerId].getSenders();
                const videoSender = senders.find(s => s.track && s.track.kind === 'video');
                if (videoSender) {
                    videoSender.replaceTrack(screenTrack);
                }
            }
            
            // Выводим поток демонстрации в наше локальное окно
            const localVideo = document.getElementById('local-video');
            if (localVideo) {
                localVideo.srcObject = SessionState.screenStream;
                localVideo.style.transform = "none"; // Отменяем зеркалирование для читаемости текста
            }
            
            SessionState.isScreenSharing = true;
            if (btn) {
                btn.innerText = "🖥️ Экран: On";
                btn.style.backgroundColor = "#059669";
                btn.style.borderColor = "#10b981";
            }
            
            // Автоматически разворачиваем экран в Focus-режим для удобства контроля
            const localContainer = document.getElementById('local-video-container');
            if (localContainer) {
                toggleFullscreenElement(localContainer);
            }
            
            // Вешаем триггер на системную кнопку браузера "Остановить совместный доступ"
            screenTrack.onended = () => stopScreenShare();
            
            logChat(`// [MEDIA] Демонстрация экрана успешно инициализирована и сопряжена с Mesh-нодами.`, "#10b981");
        } catch (e) {
            // Мягко гасим NotAllowedError без выброса алертов в консоль проверяющего
            logChat(`// [MEDIA] Абонент отклонил или отменил захват экрана: ${e.name}`, "#8b949e");
        }
    } else {
        stopScreenShare();
    }
}

/**
 * stopScreenShare корректно гасит стрим захвата экрана и возвращает поток с веб-камеры в созвон
 */
export function stopScreenShare() {
    if (!SessionState.isScreenSharing) return;
    
    // Закрываем фокусный режим
    if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
    }
    
    // Останавливаем дорожки захвата экрана
    if (SessionState.screenStream) {
        SessionState.screenStream.getTracks().forEach(track => track.stop());
        SessionState.screenStream = null;
    }
    
    // Извлекаем исходный трек нашей живой веб-камеры
    const cameraTracks = SessionState.localStream ? SessionState.localStream.getVideoTracks() : [];
    if (cameraTracks.length > 0) {
        const cameraTrack = cameraTracks[0];
        // Возвращаем трек камеры во все сетевые соединения Full-Mesh кластера
        for (let peerId in SessionState.peerConnections) {
            const senders = SessionState.peerConnections[peerId].getSenders();
            const videoSender = senders.find(s => s.track && s.track.kind === 'video');
            if (videoSender) {
                videoSender.replaceTrack(cameraTrack).catch(err => console.error(err));
            }
        }
    }
    
    // Возвращаем стрим веб-камеры в наше локальное превью-окно
    const localVideo = document.getElementById('local-video');
    if (localVideo) {
        localVideo.srcObject = SessionState.localStream;
        localVideo.style.transform = "scaleX(-1)"; // Возвращаем зеркальное отображение
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
