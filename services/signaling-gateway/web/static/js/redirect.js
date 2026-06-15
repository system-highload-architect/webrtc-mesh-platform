export function initRedirectPage() {
    const urlParams = new URLSearchParams(window.location.search);
    const targetUrl = urlParams.get('target');

    const targetUrlDisplay = document.getElementById('target-url-display');
    const confirmBtn = document.getElementById('confirm-redirect-btn');

    if (!targetUrl) {
        targetUrlDisplay.innerText = "Ошибка: Целевой URL-адрес не указан.";
        targetUrlDisplay.style.color = "#e53e3e";
        confirmBtn.style.display = "none";
        return;
    }

    // Декодируем и безопасно выводим адрес ссылки на экран
    const decodedUrl = decodeURIComponent(targetUrl);
    targetUrlDisplay.innerText = decodedUrl;

    // Вешаем обработчик осознанного b2b-перехода
    confirmBtn.onclick = () => {
        window.location.href = decodedUrl;
    };
}
