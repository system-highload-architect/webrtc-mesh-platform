import { SessionState } from '../session_context.js';

/**
 * createPeerConnection нативно генерирует изолированное P2P-плечо WebRTC для удаленного участника
 */
export async function createPeerConnection(peerId, peerName, isInitiator) {
    // Защита от дублирования: если соединение с этой нодой уже открыто, выходим
    if (SessionState.peerConnections[peerId]) return;
    SessionState.peerNames[peerId] = peerName;

    const pc = new RTCPeerConnection(SessionState.rtcConfig);
    SessionState.peerConnections[peerId] = pc;

    // Нативно подмешиваем треки нашей камеры/микрофона в создаваемый P2P-канал
    if (SessionState.localStream) {
        SessionState.localStream.getTracks().forEach(track => {
            pc.addTrack(track, SessionState.localStream);
        });
    }

    // Собираем и отправляем локальные ICE-кандидаты точечно на бэкенд шлюза сигнализации
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

    // ГЛАВНЫЙ ИНТЕРЦЕПТОР МЕДИА: Слышим входящие треки ИЗ СЕТИ от удаленного абонента
    pc.ontrack = (event) => {
        // Если видео-окно для этого ID уже отрендерено в DOM — игнорируем дубликат
        if (document.getElementById(`video-${peerId}`)) return;

        const grid = document.getElementById('video-grid');
        if (!grid) return;

        const wrapper = document.createElement('div');
        wrapper.className = "video-wrapper";
        wrapper.id = `video-${peerId}`;
        
        // Нативно вешаем обработчик полноэкранного Focus-режима (в отдельный файл позже)
        wrapper.setAttribute('ondblclick', 'toggleFullscreenElement(this)');

        // Применяем твои эталонные b2b flex-пропорции для идеальной симметрии Grid-матрицы
        wrapper.style.cssText = `
            position: relative; background: var(--bg-dark); border-radius: 8px; overflow: hidden; 
            flex: 1 1 340px; max-width: 48%; max-height: calc(50dvh - 50px); 
            aspect-ratio: 16/9; display: flex; align-items: center; justify-content: center; 
            border: 1px solid var(--border-slate); box-sizing: border-box;
        `;

        wrapper.innerHTML = `
            <video id="stream-${peerId}" autoplay playsinline style="width: 100%; height: 100%; object-fit: cover; background: var(--bg-dark); z-index: 1;"></video>
            <span class="peer-name" style="position: absolute; bottom: 8px; left: 8px; background: rgba(0, 0, 0, 0.65); color: var(--amber-primary); padding: 4px 10px; border-radius: 4px; font-size: 0.75rem; font-family: monospace; z-index: 10; pointer-events: none; border: 1px solid rgba(255, 255, 255, 0.05);">${peerName}</span>
        `;
        
        grid.appendChild(wrapper);

        // Цепляем чистый входящий СЕТЕВОЙ стрим к тегу видео (Полная изоляция от localStream!)
        const remoteVideo = document.getElementById(`stream-${peerId}`);
        if (remoteVideo) {
            remoteVideo.srcObject = event.streams[0];
        }
    };

    // Если мы инициаторы соединения (зашли в комнату вторыми) — генерируем SDP Offer
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

/**
 * removePeerVideo хирургически удаляет видео-ноду покинувшего созвон участника из DOM-дерева
 */
export function removePeerVideo(peerId) {
    const el = document.getElementById(`video-${peerId}`);
    if (el) el.remove();
    
    if (SessionState.peerConnections[peerId]) {
        SessionState.peerConnections[peerId].close();
        delete SessionState.peerConnections[peerId];
    }
}
