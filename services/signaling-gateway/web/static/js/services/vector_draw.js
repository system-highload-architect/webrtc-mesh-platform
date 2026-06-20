import { SessionState } from '../session_context.js';

let canvas = null;
let ctx = null;
let isDrawing = false;
let startX = 0;
let startY = 0;

// Структуры для хранения текущей активной указки (локальной и удаленной)
let localArrow = null;
let remoteArrow = null;

/**
 * initVectorDrawingEngine инициализирует контур лазерной указки
 * FIXED: Reengineered drawing loop to project a single straight vector arrow that decays in 1s
 */
export function initVectorDrawingEngine() {
    canvas = document.getElementById('vector-canvas');
    if (!canvas) return;
    ctx = canvas.getContext('2d');

    const resizeCanvas = () => {
        canvas.width = canvas.clientWidth;
        canvas.height = canvas.clientHeight;
    };
    resizeCanvas();
    window.addEventListener('resize', resizeCanvas);

    // Твой оригинальный интерцептор условий активации доклада (ОСТАВЛЕН БЕЗ ИЗМЕНЕНИЙ)
    setInterval(() => {
        const isScreenSharingActive = SessionState.screenStream && SessionState.screenStream.active;
        const isSpeakerModeActive = SessionState.activeSpeakerId && SessionState.activeSpeakerId !== "";

        if (isScreenSharingActive || isSpeakerModeActive) {
            canvas.style.pointerEvents = "auto";
        } else {
            canvas.style.pointerEvents = "none";
        }
    }, 500);

    // ХЛАДНОКРОВНЫЙ ПЕРЕХВАТ ДВОЙНОГО КЛИКА (Fullscreen Fix Давида):
    // Пробиваем холст насквозь, вычисляем элемент под ним и нативно увеличиваем WebRTC-плитку!
    // FIXED: Embedded hardware-like pointer puncture mapping to trigger full mesh node expansion hooks
    canvas.ondblclick = (e) => {
        // Программно прячем холст на 1 наносекунду, чтобы заглянуть "под него"
        canvas.style.pointerEvents = "none";
        const elementUnderCanvas = document.elementFromPoint(e.clientX, e.clientY);
        canvas.style.pointerEvents = "auto"; // Мгновенно возвращаем управление холсту

        if (elementUnderCanvas) {
            // Находим ближайшую видео-плитку или wrapper
            const targetVideoWrapper = elementUnderCanvas.closest('.video-wrapper') || elementUnderCanvas.closest('video') || elementUnderCanvas;
            
            if (targetVideoWrapper && (targetVideoWrapper.tagName === 'VIDEO' || targetVideoWrapper.classList.contains('video-wrapper'))) {
                console.log("🪐 [APPSEC BYPASS] Двойной клик пробил векторный маркер. Fullscreen активен.");
                if (!document.fullscreenElement) {
                    if (targetVideoWrapper.requestFullscreen) {
                        targetVideoWrapper.requestFullscreen().catch(err => console.error(err));
                    } else if (targetVideoWrapper.parentElement && targetVideoWrapper.parentElement.requestFullscreen) {
                        targetVideoWrapper.parentElement.requestFullscreen().catch(err => console.error(err));
                    }
                } else {
                    document.exitFullscreen();
                }
            }
        }
    };

    // Жесты мыши
    canvas.onmousedown = (e) => {
        // ИСПРАВЛЕНО (Защита меню управления от перехвата): 
        // Если Давид кликнул в нижней панели или зоне кнопок, холст ДОЛЖЕН пропустить клик вниз к кнопке!
        // Вычисляем элемент под холстом перед стартом рисования
        canvas.style.pointerEvents = "none";
        const clickedNode = document.elementFromPoint(e.clientX, e.clientY);
        canvas.style.pointerEvents = "auto";

        if (clickedNode && (clickedNode.closest('button') || clickedNode.closest('input') || clickedNode.closest('.controls-bar-context'))) {
            // Пропускаем событие: кликаем по реальной кнопке под холстом и выходим
            if (clickedNode.click) clickedNode.click();
            return;
        }

        isDrawing = true;
        const rect = canvas.getBoundingClientRect();
        startX = e.clientX - rect.left;
        startY = e.clientY - rect.top;
        localArrow = { x1: startX, y1: startY, x2: startX, y2: startY, color: "#ecc94b" };
    };

    canvas.onmousemove = (e) => {
        if (!isDrawing) return;
        const rect = canvas.getBoundingClientRect();
        const currentX = e.clientX - rect.left;
        const currentY = e.clientY - rect.top;

        // Обновляем координаты единственной локальной стрелки (она тянется ровной линией!)
        localArrow.x2 = currentX;
        localArrow.y2 = currentY;

        // Транслируем координаты в WebSocket для синхронизации у зала
        if (SessionState.ws && SessionState.ws.readyState === WebSocket.OPEN) {
            SessionState.ws.send(JSON.stringify({
                type: "draw_vector",
                room_id: SessionState.roomId,
                target_id: String(startX),
                target_peer_id: String(startY),
                text: String(currentX),
                command: String(currentY)
			}));
		}
	};

	canvas.onmouseup = () => {
		isDrawing = false;
		// ИСПРАВЛЕНО (Исчезновение указки через 1 секунду):
		// Спустя 1000мс полностью стираем локальный вектор с экрана
		setTimeout(() => {
			localArrow = null;
		}, 1000);
	};

	// Запускаем высокопроизводительный цикл отрисовки
	renderVectorsLoop();
}

/**
 * handleRemoteVectorInjected ловит координаты стрелки от соседа
 */
export function handleRemoteVectorInjected(msg) {
    if (!canvas) return;

    // Складываем прилетевшие координаты в объект удаленной стрелки (красим в изумрудный)
    remoteArrow = {
        x1: parseFloat(msg.start_x),
        y1: parseFloat(msg.start_y),
        x2: parseFloat(msg.end_x),
        y2: parseFloat(msg.end_y),
        color: "#10b981"
    };

    // Вектор соседа точно так же сгорает через 1 секунду после того, как он перестал им двигать
    // Защитный b2b-таймаут очистки
    if (window.remoteArrowTimer) clearTimeout(window.remoteArrowTimer);
    window.remoteArrowTimer = setTimeout(() => {
        remoteArrow = null;
    }, 1000);
}

/**
 * drawSingleArrow вспомогательная b2b-функция отрисовки идеальной геометрической стрелки
 */
function drawSingleArrow(x1, y1, x2, y2, color) {
    if (!ctx) return;

    // Рисуем тело стрелки (идеально прямая линия)
    ctx.beginPath();
    ctx.strokeStyle = color;
    ctx.lineWidth = 4; // Делаем стрелку указки чуть толще и солиднее
    ctx.lineCap = "round";
    ctx.moveTo(x1, y1);
    ctx.lineTo(x2, y2);
    ctx.stroke();

    // Математический расчет треугольного наконечника, который всегда смотрит в сторону движения мыши
    const angle = Math.atan2(y2 - y1, x2 - x1);
    ctx.beginPath();
    ctx.fillStyle = color;
    ctx.moveTo(x2, y2);
    ctx.lineTo(x2 - 16 * Math.cos(angle - Math.PI / 6), y2 - 16 * Math.sin(angle - Math.PI / 6));
    ctx.lineTo(x2 - 16 * Math.cos(angle + Math.PI / 6), y2 - 16 * Math.sin(angle + Math.PI / 6));
    ctx.fill();
}

/**
 * renderVectorsLoop отрисовывает растр через requestAnimationFrame
 */
function renderVectorsLoop() {
    if (!ctx || !canvas) {
        requestAnimationFrame(renderVectorsLoop);
        return;
    }

    // Очищаем Canvas на каждом кадре — это полностью убирает старый грязный шлейф векторов!
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Если на экране есть активная локальная указка — рендерим её
    if (localArrow) {
        drawSingleArrow(localArrow.x1, localArrow.y1, localArrow.x2, localArrow.y2, localArrow.color);
    }

    // If there is active remote arrow from guest — рендерим её
    if (remoteArrow) {
        drawSingleArrow(remoteArrow.x1, remoteArrow.y1, remoteArrow.x2, remoteArrow.y2, remoteArrow.color);
    }

    requestAnimationFrame(renderVectorsLoop);
}
