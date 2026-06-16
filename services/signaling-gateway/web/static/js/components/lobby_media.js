/**
 * captureLobbyPreview выполняет локальный захват устройств и транслирует поток в окно PREVIEW
 */
export async function captureLobbyPreview() {
    const previewVideo = document.getElementById('lobby-preview-video');
    if (!previewVideo) return null;

    try {
        // Аппаратно запрашиваем доступ к камере и микрофону у операционной системы
        const stream = await navigator.mediaDevices.getUserMedia({ 
            video: { width: 1280, height: 720 }, 
            audio: true 
        });

        // Нативно привязываем медиапоток к тегу <video> лобби-экрана
        previewVideo.srcObject = stream;
        previewVideo.style.opacity = "1";
        
        return stream;
    } catch (err) {
        console.warn("[LOBBY HARDWARE] Доступ к камере отклонен или устройство занято:", err.message);
        
        // Фолбэк-заглушка UI во избежание падения верстки на серверах без медиа-периферии
        previewVideo.style.background = "#020617";
        const container = previewVideo.parentElement;
        if (container) {
            container.innerHTML += `<span style="color:#ef4444; font-size:11px; font-family:monospace; position:absolute; top:45%; left:25%; z-index:2; text-align:center;">⚠️ АППАРАТНАЯ КАМЕРА НЕ ОБНАРУЖЕНА</span>`;
        }
        return null;
    }
}
