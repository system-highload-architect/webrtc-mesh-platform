# 🌐 Room Manager & WebSocket Signaling — Technical Specification (SRS)

### 📋 1. Core System Objectives / Основные бизнес-цели
* **[RU]** Проектирование и реализация центрального In-Memory диспетчера комнат видеоконференций. Модуль обязан изолировать контексты сессий участников, обрабатывать управляющие WebSocket-фреймы модерации (Mute, Ban, Pause) и коммутировать метаданные SDP/ICE со скоростью оперативной памяти без дискового ввода-вывода.
* **[EN]** Engineering and deployment of the central In-Memory video conference room orchestrator. The module must isolate peer session contexts, process WebSocket control moderation frames (Mute, Ban, Pause), and route SDP/ICE metadata at raw RAM speeds bypassing persistent disk storage.

### 📊 2. Memory Structures & Room Topology / Структуры данных в RAM
* **[RU]** Для полной ликвидации *Mutex Contention* под параллельным натиском тысяч WebSocket-соединений, глобальная мапа комнат разбивается на 32 независимых шарда. Внутри каждого шарда состояние комнаты описывается строго типизированными структурами:
* **[EN]** To entirely eliminate *Mutex Contention* under parallel pressure from thousands of concurrent WebSocket channels, the global room lookup matrix is split into 32 autonomous memory shards. Inside each shard, room boundaries are represented via strictly typed Go primitives:

```go
type PeerSession struct {
	PeerID           string          // Уникальный идентификатор участника
	Conn             any             // Обертка над неблокирующим сокетом gorilla.Conn
	IsModerator      bool            // Флаг b2b-прав создателя комнаты
	IsMuted          bool            // Статус блокировки микрофона модератором
	LastHeartbeat    time.Time       // Таймстамп для ленивой инвалидации сессии
}

type VideoRoom struct {
	RoomID           string          // Уникальный хэш-ключ комнаты
	PassphraseHash   string          // Соль + SHA256 пароля для входа
	MaxPeers         int32           // Лимит участников (до 100 человек по ТЗ)
	IsPaused         bool            // Стейт-флаг режима общего перерыва
	ActiveSpeakerID  string          // Текущий доминирующий спикер (VAD телеметрия)
	Peers            map[string]*PeerSession
	CreatedAt        time.Time
}
```

### ⚙️ 3. Algorithmic Logic & State Transitions / Алгоритмы и логика переходов
1. **Реактивный Даунгрейд Скорости (Quality Auto-Scaling):**
   * **[RU]** если количество активных участников `len(Peers)` превышает 15 человек, сигнальный сервер автоматически шлет команду пересогласования SDP-контракта, принудительно понижая битрейт видео для пассивных зрителей, разгружая сетевые карты клиентов.
   * **[EN]** if the active subscriber volume `len(Peers)` passes a 15-client threshold, the signaling server triggers a dynamic renegotiation of the SDP contract, automatically downgrading video bitrates for passive viewers to reduce edge network load.
2. **Физика Режима Паузы:**
   * **[RU]** при вызове владельцем команды `SET_PAUSE`, сервер переводит флаг `IsPaused = true`, блокирует аудио-дорожки участников и рассылает WebSocket-фрейм, снижающий частоту опорных кадров демонстрации экрана до 1 кадра в 5 секунд (**Muted Keyframes**), что снижает нагрузку на сетевой интерфейс сервера на 95%.
   * **[EN]** upon a moderator triggering the `SET_PAUSE` payload, the engine flips the state to `IsPaused = true`, blocks audio tracks, and fan-outs a custom frame reducing screen-share stream delivery down to 1 frame per 5 seconds (**Muted Keyframes**), compressing server interface load by 95%.
3. **Ленивое вытеснение (Паттерн Давида):**
   * **[RU]** структура комнат внутри шарда оборачивается в наш `ReactiveLruCache`. Если комната пустует дольше 30 минут, при первом же системном вызове `Get` или превышении лимита подов в 1000 единиц, запускается каскадное сжатие хвоста (Tail-to-Head Cascade Eviction). Кэш уничтожает мертвые комнаты и форсирует `runtime.GC()` для возврата страниц RAM операционной системе хоста.
   * **[EN]** room allocations inside each shard boundary are encapsulated via our `ReactiveLruCache`. If a room stays empty for over 30 minutes, upon the first `Get` query or hard capacity limits overflow past 1000 nodes, it enforces a contiguous bottom-up Cascade Eviction. The cache drops dead room states and enforces `runtime.GC()` to release memory pages back to the host OS.

### 🎖️ 4. Acceptance Criteria / Критерии приемки
* **[RU]** изоляция контекстов комнат: падение одной комнаты из-за сетевого сбоя не должно влиять на обработку соседних сессий.
* **[EN]** clean channel context isolation: any sudden connection drops or room panics must not impact neighbor active socket connections.
* **[RU]** полный отказ от глобальных блокировок мапы комнат в пользу 32-сегментного шардирования.
* **[EN]** complete omission of global resource locks across room registries in favor of a 32-way memory map sharding topology.
