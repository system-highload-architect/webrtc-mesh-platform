# 📊 Batch Chat Logger & Safe Redirect — Technical Specification (SRS)

### 📋 1. Core System Objectives / Основные бизнес-цели
* **[RU]** Асинхронный перехват, санитизация от XSS-уязвимостей и пакетная буферизация истории текстовой переписки видеоконференций. Главная задача — полностью изолировать сетевые потоки сигнального шлюза от тяжелых задержек дисковой подсистемы.
* **[EN]** Asynchronous ingestion, XSS sanitization, and batch buffering of text chat history. The main target is to strictly isolate the signaling stream path from persistent disk I/O performance degradation.

### ⚙️ 2. Algorithmic Buffering & Disk I/O Batching / Физика пакетного сброса
1. **Неблокирующий Вход (Non-blocking L4 Ingestion):**
   * **[RU]** когда участник отправляет сообщение, оно первым делом очищается от HTML-тегов и `<script>` векторов на стороне Go-сервера. После этого CDR-запись чата выстреливает в буфер `chat-history-service` на базе Go-канала емкостью 50 000 элементов по логике `select-default`, предотвращая голодание сигнальных горутин.
   * **[EN]** when a peer dispatches a text payload, it is instantly sanitized from raw HTML tags and `<script>` vectors at the Go-core boundary. Afterward, the text message frame fires to the `chat-history-service` buffer via a non-blocking Go channel featuring a 50,000 limit using `select-default` flows to block websocket stream thread starvation.
2. **Пакетный сброс (Batch INSERT):**
   * **[RU]** специальный фоновый воркер накапливает логи переписки в памяти. Запись текстового блока на диск в файл-сегмент истории происходит строго при достижении лимита в 30 сообщений ИЛИ по тайм-ауту встроенного тикера в 100 мс. Это нивелирует I/O-нагрузку на накопитель сервера.
   * **[EN]** an independent background worker aggregates active chat entries within RAM. Writing the accumulated text segment block to a persistent historical partition file executes strictly upon accumulating exactly 30 messages OR triggering a 100ms internal timer ticker, removing disk I/O wearing.
3. **Safe Redirect Interceptor:**
   * **[RU]** текст сообщения валидируется регулярными выражениями. Обнаруженные гиперссылки принудительно оборачиваются в изолированный локальный прокси-роутер, блокируя прямой переход участника на внешние фишинговые ресурсы без подтверждения на Safe Transfer Page.
   * **[EN]** message strings are evaluated via regex filters. Discovered hyperlinks are forced into a local secure proxy routing wrap, preventing direct client transfer paths to external malicious endpoints without an explicit confirmation prompt at the Safe Transfer Page.

### 🎖️ 4. Acceptance Criteria / Критерии приемки
* **[RU]** сброс истории по тайм-ауту должен отрабатывать стабильно, даже если текущий поток сообщений в чате упал до нуля.
* **[EN]** timed commit sequences must execute deterministically, even if the active input chat transaction traffic degrades to zero.
* **[RU]** серверная отсечка размера текста сообщения на лимит в 1000 символов должна происходить на уровне подсчета рун (`len([]rune(text))`), страхуя систему от переполнения буферов при обходе фронтенда.
* **[EN]** server-enforced length truncation of incoming message payloads at a 1000-character cap must scale strictly at the unicode rune layer (`len([]rune(text))`) to insulate buffers from overflow attacks bypassing frontend rules.
