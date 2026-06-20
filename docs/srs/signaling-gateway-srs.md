# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): SIGNALING GATEWAY

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Назначение и Контур Контроля Емкости (PCEF)
Шлюз `signaling-gateway` обязан оркестрировать WebRTC-сессии сопряжения пиров, хранить стейты лекционных блокировок комнат в оперативной памяти и принудительно вытеснять просроченные сессии через Битовое Колесо Времени [1.1, 2.1].

### 2. Технические параметры и защита от DoS/OOM
* **Жесткие лимиты PCEF**: Вместимость одной комнаты настраивается динамически на этапе фрейма `join` в границах от **1 до 100 участников** [2.1]. При превышении лимита шлюз обязан атомарно заблокировать сетевой сокет, выдать фрейм `room_full` и оборвать соединение, не допуская деградации CPU [2.1].
* **Лимиты Длительности Сессий (TTL)**: Максимальное время жизни встречи составляет **300 минут (5 часов)**.
* **Битовое Колесо Времени**: Логика планировщика обязана работать за $O(1)$ на базе 320-битного регистра (`uint64`), полностью исключая создание горутин-таймеров на каждую комнату [1.1]. 
* **Атомарная компенсация пауз**: Команда `SET_PAUSE` обязана заморозить время плановой смерти комнаты. При команде `RESUME_CONFERENCE` планировщик обязан вычислить точное время простоя через `time.Since` и продлить сессию в ОЗУ секунда в секунду [1.1, 2.1].

---

## 🇺🇸 ENGLISH VERSION

### 1. Scope & Capacity Management (PCEF)
The `signaling-gateway` manages volatile room states, orchestrates internal full-mesh video layouts, and garbage-collects dead node resources via hardware-like scheduling loops [1.1, 2.1].

### 2. Performance Metrics & Security Shields
* **PCEF Capacity Limits**: Hard connections quota limits scale dynamically from **1 to 100 active connections** per sharded context [2.1]. Oversubscription triggers immediate socket termination via a `room_full` frame.
* **Bitmapped Allocation Bounds**: Time wheel operations must adhere to strict zero-allocation boundaries on every minute tick, leveraging bitwise math to evaluate elapsed session deadlines [1.1].
* **Memory Isolation**: Room allocations move within 32 independent mutex-protected shards, negating multi-core thread lock contention bottlenecks.
