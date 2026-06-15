let currentSuggestion = "";

/**
 * checkT9 опрашивает версионированный v1 REST-эндпоинт префиксного дерева Trie
 * checkT9 queries the versioned v1 REST gateway of the In-Memory Trie tree
 */
export async function checkT9(inputEl, placeholderEl) {
    const words = inputEl.value.split(" ");
    const lastWord = words[words.length - 1];

    // Начинаем предиктивный поиск Т9 строго от 2-х введенных символов
    if (lastWord.length < 2) {
        placeholderEl.innerText = "";
        currentSuggestion = "";
        return "";
    }

    try {
        // Вызываем строго ВЕРСИОНИРОВАННЫЙ v1 эндпоинт API монолита (Req. 4 & 5)
        const response = await fetch(`/api/v1/t9?prefix=${encodeURIComponent(lastWord)}`);
        if (response.ok) {
            const suggestion = await response.text();
            
            // Если подсказка содержит введенный префикс (или его транслит), рендерим серый плейсхолдер
            if (suggestion && suggestion.startsWith(lastWord)) {
                currentSuggestion = suggestion;
                
                // Вычисляем точное количество пробелов для идеального наложения подсказки
                const spaces = " ".repeat(inputEl.value.length - lastWord.length);
                placeholderEl.innerText = spaces + suggestion;
                return suggestion;
            }
        }
    } catch (err) {
        console.error("[AppSec Chat] Сетевой сбой при опросе ядра Т9:", err);
    }

    placeholderEl.innerText = "";
    currentSuggestion = "";
    return "";
}

/**
 * handleTabCompletion фиксирует перенос каретки при нажатии клавиши Tab
 * handleTabCompletion handles the input manipulation when the Tab key triggers
 */
export function handleTabCompletion(inputEl, placeholderEl, suggestion) {
    if (!suggestion) return "";
    
    const words = inputEl.value.split(" ");
    words[words.length - 1] = suggestion; // Заменяем префикс на полное слово из Trie-дерева
    
    inputEl.value = words.join(" ") + " "; // Добавляем пробел для комфортного ввода следующего слова
    placeholderEl.innerText = "";
    return suggestion;
}
