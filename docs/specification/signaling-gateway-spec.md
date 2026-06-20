# 🎰 SPECIFICATION: SIGNALING GATEWAY / ШЛЮЗ СИГНАЛИЗАЦИИ WEBRTC

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ
Микросервис `signaling-gateway` (Порт `:8081`) управляет Control Plane сопряжением WebRTC-нод, контролирует лимиты емкости комнат (PCEF) и координирует лекционные блокировки треков медиа-трафика [2.1].

### 📐 Схема Full-Mesh сигнализации комнат:
```text
                       ┌─────────────────────┐
                       │  signaling-gateway  │
                       └──────────┬──────────┘
                  WebSocket       │       WebSocket
            ┌─────────────────────┴─────────────────────┐
            ▼                                           ▼
  [David (Organizer)] <--------- User Plane ---------> [Employee (Guest)]
                         (Direct WebRTC Video)
```

### 📊 Диаграмма оркестрации лекционных режимов (Orchestration Pipeline):
```mermaid
sequenceDiagram
    autonumber
    participant Organizer as 👑 David (Organizer)
    participant Gateway as 🎰 signaling-gateway
    participant Shard as 🗄️ Shard RAM Cache
    participant Guest as 📱 Employee (Guest)

    Organizer->>Gateway: WS: {"type":"control_frame","command":"GLOBAL_MUTE_AUDIO"}
    Gateway->>Shard: Lock() & Взвод булева стейта: room.RoomStates[1] = true
    Shard-->>Gateway: Стейт сохранен
    Gateway->>Gateway: Извлечение активных сетевых коннектов conns[roomID]
    Gateway-->>Guest: WS Сигнал: {"type":"force_mute_audio_lock"}
    Note over Guest: Кнопка микрофона блокируется (pointerEvents = "none")
    Gateway->>Gateway: Shard.mu.Unlock()
```

---

## 🇺🇸 ENGLISH VERSION
The `signaling-gateway` core engine (Port `:8081`) translates orchestrational events into real-time WebRTC connectivity matrices [2.1].

* **Thread-Safe Partitioning**: Cluster partitions room sessions into 32 isolated sharded mutex vectors (`RoomShard`), reducing resource contention.
* **Mute States Preservation**: Dynamic room maps record active track lock variables, instantly silencing newly connecting guest entities upon joining.
