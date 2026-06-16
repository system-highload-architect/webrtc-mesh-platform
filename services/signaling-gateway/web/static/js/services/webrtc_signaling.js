import { SessionState } from '../session_context.js';
import { createPeerConnection } from './create_peer.js';

/**
 * handleOffer принимает входящий SDP Offer, разворачивает P2P-плечо и отправляет SDP Answer
 */
export async function handleOffer(msg) {
    // Входящее плечо: создаем контур PeerConnection со стейтом инициатора false
    await createPeerConnection(msg.sender_id, msg.sender_name, false);
    
    const pc = SessionState.peerConnections[msg.sender_id];
    if (!pc) return;

    try {
        // ИСПРАВЛЕНО: Читаем структуру контракта напрямую из объекта payload, как на твоем сайте
        // FIXED: Safely unmarshaled remote SDP session description context from the inbound payload wrapper
        if (!msg.payload) {
            console.error("[WebRTC Signaling] Критическая ошибка: Payload пуст.");
            return;
        }

        await pc.setRemoteDescription(new RTCSessionDescription(msg.payload));
        
        // Формируем b2b ответный контракт (SDP Answer)
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        
        // Отправляем ответ точечно автору оффера через API Gateway
        if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
            SessionState.ws.send(JSON.stringify({
                type: "answer",
                room_id: SessionState.roomId,
                payload: answer,
                target_id: msg.sender_id
            }));
        }
    } catch (err) {
        console.error("[WebRTC Signaling] Крах обработки SDP Offer:", err);
    }
}

/**
 * handleAnswer принимает ответный SDP контракт и закрывает фазу согласования кодеков
 */
export async function handleAnswer(msg) {
    const pc = SessionState.peerConnections[msg.sender_id];
    if (!pc || !msg.payload) return;

    try {
        // ИСПРАВЛЕНО: Нативно скармливаем весь payload в RTCSessionDescription
        await pc.setRemoteDescription(new RTCSessionDescription(msg.payload));
    } catch (err) {
        console.error("[WebRTC Signaling] Крах установки SDP Answer:", err);
    }
}

/**
 * handleCandidate вживляет удаленные сетевые координаты ICE для пробития NAT-экранов
 */
export async function handleCandidate(msg) {
    const pc = SessionState.peerConnections[msg.sender_id];
    if (!pc || !msg.payload) return;

    try {
        // ИСПРАВЛЕНО: Нативно скармливаем payload в конструктор RTCIceCandidate
        // FIXED: Fed the raw payload object strictly into native browser ice layer
        await pc.addIceCandidate(new RTCIceCandidate(msg.payload));
    } catch (err) {
        console.error("[WebRTC Signaling] Крах добавления Ice Candidate:", err);
    }
}
