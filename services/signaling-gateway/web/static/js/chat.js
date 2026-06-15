let currentSuggestion = "";

export async function checkT9(inputEl, placeholderEl) {
    const words = inputEl.value.split(" ");
    const lastWord = words[words.length - 1];

    if (lastWord.length < 2) {
        placeholderEl.innerText = "";
        currentSuggestion = "";
        return "";
    }

    // Вызываем строго ВЕРСИОНИРОВАННЫЙ v1 эндпоинт API
    const response = await fetch(`/api/v1/t9?prefix=${encodeURIComponent(lastWord)}`);
    if (response.ok) {
        const suggestion = await response.text();
        if (suggestion && suggestion.startsWith(lastWord)) {
            currentSuggestion = suggestion;
            const spaces = " ".repeat(inputEl.value.length - lastWord.length);
            placeholderEl.innerText = spaces + suggestion;
            return suggestion;
        }
    }
    placeholderEl.innerText = "";
    currentSuggestion = "";
    return "";
}

export function handleTabCompletion(inputEl, placeholderEl, suggestion) {
    if (!suggestion) return "";
    const words = inputEl.value.split(" ");
    words[words.length - 1] = suggestion;
    inputEl.value = words.join(" ") + " ";
    placeholderEl.innerText = "";
    return suggestion;
}
