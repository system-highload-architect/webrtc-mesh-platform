import { SessionState } from '../session_context.js';

function getTilesContainer() {
    return document.getElementById('video-tiles') || document.getElementById('video-grid');
}

function getStageContainer() {
    return document.getElementById('speaker-stage');
}

function getAllVideoTiles() {
    const tiles = getTilesContainer();
    const stage = getStageContainer();
    const nodes = [];

    if (tiles) {
        tiles.querySelectorAll('.video-wrapper, #local-video-container').forEach((node) => nodes.push(node));
    }
    if (stage) {
        stage.querySelectorAll('.video-wrapper, #local-video-container').forEach((node) => nodes.push(node));
    }

    return nodes;
}

function ensureVideoPlayback(tile) {
    const video = tile ? tile.querySelector('video') : null;
    if (video && video.srcObject) {
        video.play().catch(() => {});
    }
}

export function getPeerVideoTile(peerId) {
    if (peerId === SessionState.myPeerId) {
        return document.getElementById('local-video-container');
    }
    return document.getElementById(`video-${peerId}`);
}

export function applySpeakerFocus(targetPeerId) {
    const grid = document.getElementById('video-grid');
    const stage = getStageContainer();
    const tiles = getTilesContainer();
    const target = getPeerVideoTile(targetPeerId);

    if (!grid || !stage || !tiles || !target) {
        return false;
    }

    getAllVideoTiles().forEach((tile) => {
        tile.classList.remove('active-speaker-focus');
        if (tile.parentElement === stage) {
            tiles.appendChild(tile);
        }
    });

    target.classList.add('active-speaker-focus');
    stage.appendChild(target);
    grid.classList.add('speaker-mode-active');
    stage.setAttribute('aria-hidden', 'false');

    SessionState.activeSpeakerId = targetPeerId;
    ensureVideoPlayback(target);
    return true;
}

export function resetSpeakerLayout() {
    const grid = document.getElementById('video-grid');
    const stage = getStageContainer();
    const tiles = getTilesContainer();

    if (!grid || !stage || !tiles) {
        return;
    }

    while (stage.firstChild) {
        const tile = stage.firstChild;
        tile.classList.remove('active-speaker-focus');
        tiles.appendChild(tile);
    }

    grid.classList.remove('speaker-mode-active');
    stage.setAttribute('aria-hidden', 'true');
    SessionState.activeSpeakerId = '';
}

export function onPeerVideoTileRemoved(peerId) {
    if (SessionState.activeSpeakerId && SessionState.activeSpeakerId === peerId) {
        resetSpeakerLayout();
    }
}
