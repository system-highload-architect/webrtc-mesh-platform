# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): SPR STORAGE

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Назначение и Архитектура Хранилища
Микросервис `spr-storage` обязан обеспечивать отказоустойчивую потоковую укладку и раздачу бинарных медиаданных WebM (Симуляция NoSQL ScyllaDB), работая в режиме Zero-Allocation (без паразитных аллокаций памяти) [2.1].

### 2. Технические ограничения и Потоковые метрики
* **gRPC Client Streaming**: сервис обязан принимать входящие видеоданные через открытый бинарный gRPC-стрим `StreamMediaChunk` [2.1]. Нарезка потока на стороне прокси-сервера жестко фиксируется буфером в **64 Килобайта** [2.1].
* **Дисковый лимит (AppSec Max Payload)**: максимальный размер одного запечатываемого WebM-монолита ограничен **500 Мегабайтами** [2.1]. Попытка превысить лимит должна аппаратно обрываться на уровне прокси через `http.MaxBytesReader` [2.1].
* **Range-Streaming Контракт**: сервер обязан отдавать медиафайлы по запросу API Gateway с поддержкой HTTP-заголовков `Accept-Ranges: bytes`, обеспечивая мгновенную побайтовую перемотку таймлайна видео в браузерах пользователей [2.1].

---

## 🇺🇸 ENGLISH VERSION

### 1. System Scope & Storage Blueprint
The `spr-storage` distributed microservice acts as a high-throughput binary blob engine (ScyllaDB emulation tier), ingesting raw WebM streams straight via gRPC [2.1].

### 2. Input Pipeline Bounds & Telemetry SLAs
* **Stream Chunking Constraints**: storage accepts inbound buffers via `StreamMediaChunk` context with a mandatory **64 KB frame payload slice** enforced by the gateway [2.1].
* **Direct I/O Seeking**: serves monolithic WebM objects using granular file descriptor seek capabilities to support browser `Accept-Ranges` byte-accurate tracking [2.1].
