/**
 * initRedirectPage извлекает целевой URL назначения и активирует b2b AppSec шилд
 * initRedirectPage parses the target payload and handles safe routing confirmations
 */
export function initRedirectPage() {
    const urlParams = new URLSearchParams(window.location.search);
    const targetUrl = urlParams.get('target');

    const targetUrlDisplay = document.getElementById('target-url-display');
    const confirmBtn = document.getElementById('confirm-redirect-btn');

    if (!targetUrl) {
        if (targetUrlDisplay) {
            targetUrlDisplay.innerText = "Ошибка AppSec: Целевой URL-адрес назначения не указан.";
            targetUrlDisplay.style.color = "#e53e3e";
        }
        if (confirmBtn) {
            confirmBtn.style.display = "none";
        }
        return;
    }

    try {
        // Безопасно декодируем URL-адрес из Query строки
        const decodedUrl = decodeURIComponent(targetUrl);
        
        if (targetUrlDisplay) {
            targetUrlDisplay.innerText = decodedUrl;
        }

        if (confirmBtn) {
            // Навешиваем обработчик осознанного перехода по внешней ссылке
            confirmBtn.onclick = () => {
                window.location.href = decodedUrl;
            };
        }
    } catch (err) {
        console.error("[Safe Shield] Ошибка декодирования URLPayload:", err);
        if (targetUrlDisplay) {
            targetUrlDisplay.innerText = "Ошибка: Некорректный формат ссылки.";
        }
    }
}
