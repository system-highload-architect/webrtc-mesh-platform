import { SessionState } from '../session_context.js';

/**
 * toggleFullscreenElement разворачивает или сворачивает выбранный HTML-элемент видео
 */
export function toggleFullscreenElement(element) {
    // Предотвращаем рекурсию кодеков при попытке развернуть локальный экран во время трансляции
    if (element.id === 'local-video-container' && SessionState.isScreenSharing) {
        return;
    }

    if (!document.fullscreenElement) {
        element.requestFullscreen()
        .then(() => {
            const exitBtn = document.getElementById('fs-exit-btn');
            if (exitBtn) exitBtn.style.display = 'block';
        })
        .catch(err => {
            console.error("[UX Fullscreen] Не удалось перевести ноду в полноэкранный режим:", err.message);
        });
    } else {
        exitFullscreenMode();
    }
}

/**
 * exitFullscreenMode нативно закрывает полноэкранный режим
 */
export function exitFullscreenMode() {
    if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
    }
}

// Глобальный слушатель изменения состояния Fullscreen для скрытия кнопки-подсказки при выходе по Esc
document.addEventListener('fullscreenchange', () => {
    const exitBtn = document.getElementById('fs-exit-btn');
    if (!document.fullscreenElement && exitBtn) {
        exitBtn.style.display = 'none';
    }
});

// Экспортируем функцию выхода в глобальный контекст window, так как она вызывается из инлайнового onclick в meet.html
window.exitFullscreenMode = exitFullscreenMode;
// Экспортируем также toggleFullscreenElement для корректной работы inline-атрибутов ondblclick на создаваемых video-wrapper
window.toggleFullscreenElement = toggleFullscreenElement;
