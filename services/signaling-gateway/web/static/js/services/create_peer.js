import { SessionState } from '../session_context.js';

/**
 * createPeerConnection нативно генерирует P2P-плечо WebRTC для удаленного участника
 */
export async function createPeerConnection(peerId, peerName, isInitiator) {
    if (SessionState.peerConnections[peerId]) return;
    SessionState.peerConnections[peerId] = true; // Защита от race-condition гонки дубликатов
    SessionState.peerNames[peerId] = peerName;

    const strictIceConfig = {
        iceServers: [{ urls: ["stun:stun.l.google.com:19302"] }]
    };

    let pc;
    try {
        pc = new RTCPeerConnection(strictIceConfig);
        SessionState.peerConnections[peerId] = pc;
    } catch (err) {
        console.error("[WebRTC Core] Критическое падение конструктора:", err.message);
        return;
    }

    if (SessionState.localStream) {
        SessionState.localStream.getTracks().forEach(track => {
            pc.addTrack(track, SessionState.localStream);
        });
    }

    pc.onicecandidate = (event) => {
        if (event.candidate && SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
            SessionState.ws.send(JSON.stringify({
                type: "candidate",
                room_id: SessionState.roomId,
                payload: event.candidate,
                target_id: peerId
            }));
        }
    };

    pc.ontrack = (event) => {
        if (document.getElementById(`video-${peerId}`)) return;

        // ИСПРАВЛЕНО (Фикс исчезновения участников ТЗ): Направляем плитки гостей строго в b2b-колодец видео-тайлов!
        // Теперь при активации спикера они гарантированно ужмутся в правый вертикальный сайдбар!
        // FIXED: Re-routed remote tracks target injection container node from global grid to video-tiles pool
        const tilesContainer = document.getElementById('video-tiles') || document.getElementById('video-grid');
        if (!tilesContainer) return;

        const wrapper = document.createElement('div');
        wrapper.className = "video-wrapper";
        wrapper.id = `video-${peerId}`;
        wrapper.setAttribute('ondblclick', 'toggleFullscreenElement(this)');
        wrapper.style.cssText = "position: relative;";

        let leftCornerButtonsHtml = "";
        let rightCornerButtonsHtml = "";
        
        if (SessionState.isModerator) {
            leftCornerButtonsHtml = `
                <div style="position: absolute; top: 8px; left: 8px; z-index: 20;">
                    <button class="target-speaker-btn" data-peer="${peerId}" style="background: rgba(15,23,42,0.85); border: 1px solid #ecc94b; color: #ecc94b; padding: 4px 8px; font-size: 10px; border-radius: 4px; cursor: pointer; font-family: monospace; font-weight: bold; transition: all 0.2s;">⭐ Спикер</button>
                </div>
            `;

            rightCornerButtonsHtml = `
                <div style="position: absolute; top: 8px; right: 8px; display: flex; gap: 4px; z-index: 20;">
                    <button class="target-mute-btn" data-peer="${peerId}" style="background: rgba(15,23,42,0.85); border: 1px solid #334155; color: #fff; padding: 4px 8px; font-size: 10px; border-radius: 4px; cursor: pointer; font-family: monospace;">🎙️ Mute</button>
                    <button class="target-kick-btn" data-peer="${peerId}" style="background: rgba(127,29,29,0.85); border: 1px solid #ef4444; color: #fff; padding: 4px 8px; font-size: 10px; border-radius: 4px; cursor: pointer; font-family: monospace;">❌ Kick</button>
                </div>
            `;
        }

        wrapper.innerHTML = `
            ${leftCornerButtonsHtml}
            ${rightCornerButtonsHtml}
            <video id="stream-${peerId}" autoplay playsinline style="width: 100%; height: 100%; object-fit: contain; background: var(--bg-dark); z-index: 1;"></video>
            <span class="peer-name" style="position: absolute; bottom: 8px; left: 8px; background: rgba(0, 0, 0, 0.65); color: var(--blue-primary); padding: 4px 10px; border-radius: 4px; font-size: 0.75rem; font-family: monospace; z-index: 10; pointer-events: none; border: 1px solid rgba(255, 255, 255, 0.05);">${peerName}</span>
        `;
        tilesContainer.appendChild(wrapper);

        const remoteVideo = document.getElementById(`stream-${peerId}`);
        if (remoteVideo && event.streams && event.streams.length > 0) {
            remoteVideo.srcObject = event.streams[0];
        }
    };

    if (isInitiator) {
        try {
            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);

            if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
                SessionState.ws.send(JSON.stringify({
                    type: "offer",
                    room_id: SessionState.roomId,
                    payload: offer,
                    target_id: peerId
                }));
            }
        } catch (err) {
            console.error("[WebRTC Core] Крах генерации локального SDP оффера:", err);
        }
    }
}

export function removePeerVideo(peerId) {
    const el = document.getElementById(`video-${peerId}`);
    if (el) el.remove();
    if (SessionState.peerConnections[peerId] && SessionState.peerConnections[peerId] !== true) {
        SessionState.peerConnections[peerId].close();
        delete SessionState.peerConnections[peerId];
    }
}
