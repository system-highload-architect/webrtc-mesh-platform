/**
 * logChat рендерит текстовые фреймы в изолированный b2b-терминал логов
 * logChat appends and formats text log lines into the shared terminal viewport
 */
export function logChat(text, color = "#ffffff") {
    const box = document.getElementById('chat-box');
    if (!box) return;

    const p = document.createElement('p');
    p.style.color = color;
    p.style.margin = "0";
    p.style.lineHeight = "1.4";
    p.style.fontFamily = "monospace";

    let formattedText = text;

    // AppSec корпоративный перехватчик внешних фишинговых линков (Req. 5)
    if (text.includes("redirect?target=")) {
        const parts = text.split("target=");
        const extractedUrl = decodeURIComponent(parts[1] || "");
        formattedText = `<span class="safe-link-trigger" data-url="${extractedUrl}" style="color: #ecc94b; text-decoration: underline; cursor: pointer; font-weight: bold;">[🔒 БЕЗОПАСНАЯ ПРОВЕРКА ССЫЛКИ]</span>`;
    }

    p.innerHTML = formattedText;
    box.appendChild(p);

    // Атомарно смещаем каретку скроллбара на самый нижний фрейм
    box.scrollTop = box.scrollHeight;

    // Нативно навешиваем изолированные клики на AppSec-линки во избежание inline-сдвигов
    const links = p.querySelectorAll('.safe-link-trigger');
    links.forEach(link => {
        link.onclick = () => {
            const targetUrl = link.getAttribute('data-url');
            window.open(`/api/v1/redirect?target=${encodeURIComponent(targetUrl)}`, '_blank');
        };
    });
}
