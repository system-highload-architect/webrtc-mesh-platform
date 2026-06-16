/**
 * handleOrganizerLogin проверяет b2b-паспорт Давида и открывает секцию провижнинга комнат
 */
export function handleOrganizerLogin() {
    const emailEl = document.getElementById('auth-email');
    const passwordEl = document.getElementById('auth-password');
    const statusEl = document.getElementById('auth-status');
    const setupSection = document.getElementById('setup-section');

    if (!emailEl || !passwordEl || !statusEl || !setupSection) return;

    const email = emailEl.value.trim();
    const password = passwordEl.value;

    // Строгая b2b-проверка паспорта модератора
    if (email === "david@clearway.ru" && password === "admin123") {
        statusEl.innerText = "// STATE: IDENTITY ВЕРИФИЦИРОВАН. ДОСТУП К КЛАТЕРУ ОТКРЫТ.";
        statusEl.style.color = "#10b981"; // Зелёный неоновый статус безопасности

        // Плавно открываем Раздел №2 (Компиляция комнаты)
        setupSection.style.display = "flex";
        
        // Блокируем поля авторизации, фиксируя стейт защиты
        emailEl.disabled = true;
        passwordEl.disabled = true;
        document.getElementById('login-btn-action').disabled = true;
    } else {
        statusEl.innerText = "// STATE: КРАХ АВТОРИЗАЦИИ. ОТКАЗАНО В ДОСТУПЕ.";
        statusEl.style.color = "#ef4444"; // Красный алерт AppSec
        
        emailEl.style.borderColor = "#ef4444";
        passwordEl.style.borderColor = "#ef4444";
    }
}
