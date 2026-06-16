/**
 * checkT9 опрашивает префиксное дерево Trie на бэкенде и выводит серую подсказку
 */
export async function checkT9(inputEl, placeholderEl) {
    const value = inputEl.value;
    const lastWord = value.split(/\s+/).pop();

    // Если слово слишком короткое, зачищаем плашку подсказки и выходим
    if (!lastWord || lastWord.length < 2) {
        placeholderEl.innerText = "";
        return "";
    }

    try {
        // Опрашиваем бэкенд через API Gateway прокси-канал (:8080)
        const response = await fetch(`/api/v1/t9?prefix=${encodeURIComponent(lastWord)}`);
        const suggestion = await response.text();

        if (suggestion && suggestion.startsWith(lastWord)) {
            // Вычисляем, сколько букв осталось дописать до целого слова
            const remainder = suggestion.slice(lastWord.length);
            
            // Нативно формируем отступ из пробелов, чтобы подсказка встала ровно встык за курсором
            const spaces = " ".repeat(value.length);
            placeholderEl.innerText = spaces + remainder;
            return suggestion;
        } else {
            placeholderEl.innerText = "";
        }
    } catch (e) {
        placeholderEl.innerText = "";
    }
    return "";
}

/**
 * handleTabCompletion нативно дописывает слово в инпут при нажатии клавиши Tab
 */
export function handleTabCompletion(inputEl, placeholderEl, activeSuggestion) {
    const words = inputEl.value.split(/\s+/);
    words.pop(); // Удаляем недописанный огрызок слова
    
    words.push(activeSuggestion); // Впрыскиваем полное слово из префиксного дерева
    
    inputEl.value = words.join(" ") + " ";
    placeholderEl.innerText = "";
}
