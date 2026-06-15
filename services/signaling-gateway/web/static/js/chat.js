let currentSuggestion = "";

/**
 * checkT9 опрашивает версионированный v1 gRPC/REST мост префиксного дерева Trie
 * checkT9 queries the versioned v1 gRPC/REST bridge of the In-Memory Trie tree
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
        // Вызываем строго ВЕРСИОНИРОВАННЫЙ v1 эндпоинт API (Req. 4 & 5)
        const response = await fetch(`/api/v1/t9?prefix=${encodeURIComponent(lastWord)}`);
        if (response.ok) {
            const suggestion = await response.text();
            
            // Если подсказка содержит введенный префикс, рендерим серый плейсхолдер
            if (suggestion && suggestion.startsWith(lastWord)) {
                currentSuggestion = suggestion;
                
                // Вычисляем точное количество пробелов для идеального наложения подсказки поверх инпута
                const spaces = " ".repeat(inputEl.value.length - lastWord.length);
                placeholderEl.innerText = spaces + suggestion;
                return suggestion;
            }
        }
    } catch (err) {
        console.error("[AppSec Chat] Крах сети при опросе Trie Т9 ядра:", err);
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
    
    inputEl.value = words.join(" ") + " "; // Добавляем b2b пробел для комфортного ввода следующего слова
    placeholderEl.innerText = "";
    return suggestion;
}
