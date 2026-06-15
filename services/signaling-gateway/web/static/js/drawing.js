let canvas = null;
let ctx = null;
let isDrawing = false;
let startX = 0;
let startY = 0;

/**
 * initDrawingEngine инициализирует b2b-контур векторного рисования стрелок поверх холста (Req. 3)
 */
export function initDrawingEngine(canvasId, ws, isModerator) {
    canvas = document.getElementById(canvasId);
    if (!canvas) return;
    ctx = canvas.getContext('2d');

    // Подгоняем физический размер разрешения холста под размеры элемента на экране
    resizeCanvas();
    window.addEventListener('resize', resizeCanvas);

    if (!isModerator) {
        // Участники сессии имеют только Read-Only доступ, рисовать мыкой они не могут
        return;
    }

    // Включаем перехват b2b-событий мыши для Модератора
    canvas.style.pointerEvents = "auto";
    canvas.style.cursor = "crosshair";

    canvas.onmousedown = (e) => {
        isDrawing = true;
        const rect = canvas.getBoundingClientRect();
        startX = e.clientX - rect.left;
        startY = e.clientY - rect.top;
    };

    canvas.onmousemove = (e) => {
        if (!isDrawing) return;
        const rect = canvas.getBoundingClientRect();
        const currentX = e.clientX - rect.left;
        const currentY = e.clientY - rect.top;

        // Рисуем стрелку локально в реальном времени (Превью)
        drawArrowOnCanvas(startX, startY, currentX, currentY, "#ecc94b", 3);
    };

    canvas.onmouseup = (e) => {
        if (!isDrawing) return;
        isDrawing = false;
        
        const rect = canvas.getBoundingClientRect();
        const endX = e.clientX - rect.left;
        const endY = e.clientY - rect.top;

        // ВЫСТРЕЛИВАЕМ ВЕКТОРНЫЕ КООРДИНАТЫ В WEBSOCKET (Control Plane Routing)
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({
                type: "sdp_offer", // Переиспользуем медиа-мост для веерной рассылки дата-фреймов
                target_peer_id: "User_Guest", // Веерно пушим всем участникам
                draw_vector: {
                    x1: startX, y1: startY,
                    x2: endX, y2: endY,
                    color: "#ecc94b",
                    width: 3
                }
            }));
        }
    };
}

/**
 * renderIncomingVector отрисовывает стрелку на стороне пассивных сотрудников (Req. 3)
 */
export function renderIncomingVector(vector) {
    if (!ctx) return;
    drawArrowOnCanvas(vector.x1, vector.y1, vector.x2, vector.y2, vector.color, vector.width);
}

function drawArrowOnCanvas(x1, y1, x2, y2, color, width) {
    if (!ctx) return;
    
    // Очищаем старое превью перед финальной фиксацией (Опционально для чистых стрелок)
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    ctx.strokeStyle = color;
    ctx.fillStyle = color;
    ctx.lineWidth = width;
    ctx.lineCap = "round";

    // 1. Рисуем главное тело ровной линии
    ctx.beginPath();
    ctx.moveTo(x1, y1);
    ctx.lineTo(x2, y2);
    ctx.stroke();

    // 2. ВЫЧИСЛЯЕМ ТРИГОНОМЕТРИЮ НАКОНЕЧНИКА СТРЕЛКИ (b2b Математика геометрии)
    const angle = Math.atan2(y2 - y1, x2 - x1);
    const arrowHeadLength = 15; // Длина усов стрелки

    ctx.beginPath();
    ctx.moveTo(x2, y2);
    ctx.lineTo(x2 - arrowHeadLength * Math.cos(angle - Math.PI / 6), y2 - arrowHeadLength * Math.sin(angle - Math.PI / 6));
    ctx.lineTo(x2 - arrowHeadLength * Math.cos(angle + Math.PI / 6), y2 - arrowHeadLength * Math.sin(angle + Math.PI / 6));
    ctx.closePath();
    ctx.fill();
}

function resizeCanvas() {
    if (!canvas) return;
    canvas.width = canvas.parentElement.clientWidth - 40;
    canvas.height = 180;
}
