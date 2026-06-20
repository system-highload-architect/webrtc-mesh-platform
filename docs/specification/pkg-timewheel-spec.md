# ⏰ FUNCTION SPECIFICATION: BITMAPPED TIME WHEEL / БИТОВОЕ КОЛЕСО ВРЕМЕНИ ТАЙМАУТОВ

## 🇷🇺 РУССКАЯ ВЕРСИЯ
Компонент `internal/pkg/timewheel/tw.go` реализует 300-битный безэлокационный планировщик таймаутов, инкапсулированный в массив `[5]uint64` [1.1].

### Архитектура регистров битовой маски в ОЗУ:
```text
  Индекс слова = Слот / 64
  Индекс бита  = Слот % 64
  
  [tw.slots] ➔ [ uint64 Word 0 ] ➔ [0100000000...00] (Бит 1 взведен ➔ 1-я минута активна)
                 [ uint64 Word 1 ]
                 [ uint64 Word 2 ]
```

### Диаграмма жизненного цикла тиков Janitor-демона ($O(1)$):
```mermaid
sequenceDiagram
    autonumber
    participant Janitor as 🎰 janitor_business.go
    participant TW as ⏰ BitmappedTimeWheel
    participant Shard as 🗄️ RoomShard RAM Cache

    Janitor->>TW: Tick() (Смещение стрелки tw.pointer++)
    TW->>TW: Расчет слова: wordIdx = pointer / 64, bitIdx = pointer % 64
    TW->>TW: Битовая маска: (slots[wordIdx] & (1 << bitIdx))
    alt Бит равен 0 (Слот пуст)
        TW-->>Janitor: Return (nil, currentPointer) за 1 такт CPU!
    else Бит равен 1 (Срок сессии истек)
        TW->>TW: Извлечь ID комнат из tw.buckets[currentPointer]
        TW->>TW: Сбросить бит в нуль: slots[wordIdx] &= ^(1 << bitIdx)
        TW-->>Janitor: Return ([]string{roomID}, currentPointer)
        Janitor->>Shard: Lock() & Принудительное вытеснение комнаты из памяти
        Janitor->>Janitor: Вызов runtime.GC()
    end
```

---

## 🇺🇸 ENGLISH VERSION
Component `internal/pkg/timewheel/tw.go` delegates session expiration tasks to raw bitwise operations.

* Addition sets the exact offset bit via `|= (1 << bitIdx)`.
* Every minute the janitor evaluates the active word block via atomic bit masking, triggering zero allocations if no sessions expired.
