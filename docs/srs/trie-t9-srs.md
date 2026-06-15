# 🛡️ In-Memory Trie Engine & Keyboard Translit — Technical Specification (SRS)

### 📋 1. Core System Objectives / Основные бизнес-цели
* **[RU]** Проектирование ультра-быстрого, наносекундного движка Т9 автодополнения сообщений в чате конференции. Система должна возвращать релевантные подсказки слов на лету, полностью исключая линейный перебор словаря $O(N \times M)$ и предотвращая аллокации памяти в куче Go на горячем пути поиска.
* **[EN]** Architectural review and implementation of a near-instantaneous T9 chat autocomplete engine. The system must yield predictive text suggestions on the fly, entirely removing linear $O(N \times M)$ dictionary iteration and guaranteeing zero runtime Go heap allocations on the hot lookup path.

### 📊 2. Algorithmic Data Structures / Структура префиксного дерева в памяти
* **[RU]** Взаимодействие строится вокруг графа префиксного дерева (**Trie Data Structure**). Каждая руна (`rune`) входящей строки является указателем на дочерний узел:
* **[EN]** Interaction patterns resolve around an **In-Memory Prefix Tree (Trie Data Structure)**. Each independent unicode character (`rune`) within the parsed string acts as a pointer to its nested leaf child:

```go
type TrieNode struct {
	Children    map[rune]*TrieNode // Ссылки на дочерние символы
	IsWord      bool               // Флаг окончания легитимного b2b-слова
	Frequency   uint64             // Счетчик популярности слова для ранжирования Т9
}
```

### ⚙️ 3. Hot Path Execution & Layout Normalization / Алгоритмы и раскладка
1. **Наносекундный поиск $O(K)$:**
   * **[RU]** движок принимает введенный пользователем префикс (например, `при`). Проход по дереву выполняется строго посимвольно за время, пропорциональное длине введенной строки $K$, минуя размер общего словаря терминов.
   * **[EN]** the engine consumes a user-submitted string prefix (e.g., `при`). Iteration over tree leafs scales strictly byte-by-byte within a deterministic complexity of $K$ (where $K$ matches the prefix string length), completely independent from total dictionary capacity bounds.
2. **Конвейер нормализации раскладки (Keyboard Layout Translit):**
   * **[RU]** если посимвольный поиск выдает `false`, Go-ядро не прекращает вычисления. Строка пропускается сквозь плоскую хэш-мапу рун-переводчиков. Ошибочный латинский ввод `ghbdtn` за один проход конвертируется в кириллический `привет`, после чего запускается повторный Trie-поиск.
   * **[EN]** if character lookups yield a `false` match, the Go-core enforces a fallback pipeline. The token passes via flat rune layout mappings. Misaligned Latin inputs like `ghbdtn` translate into Cyrillic `привет` within a single run pass before executing a secondary Trie lookup.
3. **Физика Автодополнения (`Tab` Trigger):**
   * **[RU]** если совпадение найдено, сервер возвращает префикс и суффикс подсказки. Фронтенд визуализирует её серым плейсхолдером. Текст подставляется в форму ввода строго по перехвату нажатия клавиши `Tab`.
   * **[EN]** if a predictive match succeeds, the server returns the remaining suffix substring. The frontend renders it as a muted gray placeholder inline. The string text auto-injects into the active input element strictly upon capturing a `Tab` keyboard interaction.

### 🎖️ 4. Acceptance Criteria / Критерии приемки
* **[RU]** время поиска автодополнения слова среди b2b-словаря емкостью 50 000 слов не должно превышать 15 наносекунд.
* **[EN]** dictionary autocomplete discovery latency targeting a 50,000 corporate vocabulary cap must not exceed 15 nanoseconds.
* **[RU]** тест бенчмарков `go test -bench` обязан фиксировать жесткий профиль утилизации памяти `0 B/op` и `0 allocs/op`.
* **[EN]** micro-benchmarks via `go test -bench` must enforce a rigid memory profile of exactly `0 B/op` and `0 allocs/op`.
