/**
 * session_context.js инкапсулирует атомарный контекст состояния Mesh-ноды в памяти
 */
export const SessionState = {
    ws: null,
    roomId: "default-room",
    myPeerId: "",
    username: "Гость",
    isModerator: false,
    
    localStream: null,
    screenStream: null,
    
    isAudioMuted: false,
    isVideoMuted: false,
    isScreenSharing: false,
    isRecording: false,
    isPaused: false,
    
    peerConnections: {}, // peerId -> RTCPeerConnection
    peerNames: {},        // peerId -> string (displayName)
    rtcConfig: { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] }
};
