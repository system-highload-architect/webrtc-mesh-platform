/**
 * setupLinkInteraction навешивает нативные b2b клик-события копирования на инпуты ссылок
 */
export function setupLinkInteraction() {
    const modLinkEl = document.getElementById('mod-link');
    const peerLinkEl = document.getElementById('peer-link');
    const enterBtn = document.getElementById('enter-mod-btn-action');

    if (!modLinkEl || !peerLinkEl || !enterBtn) return;

    // Нативная функция клик-копирования линка в системный буфер обмена OS
    const bindCopyOnClick = (element, labelText) => {
        element.onclick = async () => {
            try {
                await navigator.clipboard.writeText(element.value);
                
                // Временный визуал-статус успешного копирования (UX комплаенс)
                const originalValue = element.value;
                element.value = `✅ Ссылка скопирована в буфер обмена!`;
                element.style.borderColor = "#10b981";

                setTimeout(() => {
                    element.value = originalValue;
                    element.style.borderColor = "#30363d";
                }, 1200);
            } catch (err) {
                console.error("Не удалось записать токен ссылки в clipboard:", err);
            }
        };
    };

    bindCopyOnClick(modLinkEl, "Модератор");
    bindCopyOnClick(peerLinkEl, "Сотрудник");

    // Финальный b2b редирект в рабочую плоскость WebRTC Mesh созвона
    enterBtn.onclick = () => {
        if (modLinkEl.value && !modLinkEl.value.includes("скопирована")) {
            window.location.href = modLinkEl.value;
        }
    };
}
