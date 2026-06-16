// Изолированный in-memory стейт кнопок лобби до момента входа в комнату
export const LobbyState = {
    isAudioMuted: false,
    isVideoMuted: false
};

/**
 * toggleLobbyAudio переключает состояние звуковой дорожки в лобби
 */
export function toggleLobbyAudio() {
    LobbyState.isAudioMuted = !LobbyState.isAudioMuted;
    
    // Считываем ссылку на стрим, сохраненную в window на этапе медиа-захвата
    const stream = window.lobbyStreamRef;
    if (stream) {
        stream.getAudioTracks().forEach(t => t.enabled = !LobbyState.isAudioMuted);
    }

    const btn = document.getElementById('lobby-audio-toggle');
    if (btn) {
        btn.innerText = LobbyState.isAudioMuted ? "🎤 Микрофон: выкл" : "🎤 Микрофон: вкл";
        btn.style.backgroundColor = LobbyState.isAudioMuted ? "#7f1d1d" : "#1e293b";
        btn.style.borderColor = LobbyState.isAudioMuted ? "#ef4444" : "#334155";
    }
}

/**
 * toggleLobbyVideo переключает состояние видеотрека и прозрачность окна предпросмотра
 */
export function toggleLobbyVideo() {
    LobbyState.isVideoMuted = !LobbyState.isVideoMuted;
    
    const stream = window.lobbyStreamRef;
    if (stream) {
        stream.getVideoTracks().forEach(t => t.enabled = !LobbyState.isVideoMuted);
    }

    const btn = document.getElementById('lobby-video-toggle');
    const previewVideo = document.getElementById('lobby-preview-video');
    
    if (btn) {
        btn.innerText = LobbyState.isVideoMuted ? "📷 Камера: выкл" : "📷 Камера: вкл";
        btn.style.backgroundColor = LobbyState.isVideoMuted ? "#7f1d1d" : "#1e293b";
        btn.style.borderColor = LobbyState.isVideoMuted ? "#ef4444" : "#334155";
    }
    
    if (previewVideo) {
        // Если камера выключена, плавно приглушаем экран, убирая "замерзший" кадр
        previewVideo.style.opacity = LobbyState.isVideoMuted ? "0.15" : "1";
    }
}
