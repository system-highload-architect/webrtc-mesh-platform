# 🗄️ SPECIFICATION: SPR STORAGE / РАСПРЕДЕЛЕННЫЙ NVMe РЕКОРДЕР

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ
Микросервис `spr-storage` (Порт `:50060`) реализует высокопроизводительное бинарное хранилище видеоданных (Эмулятор ScyllaDB NoSQL) и принимает Client Streaming потоки кадров WebM напрямую по gRPC-каналам [2.1].

### 📐 Схема нарезки и сборки WebM монолита на NVMe-массиве:
```text
  [HTTP POST Чанки 64КБ] ➔ proxy ➔ gRPC Stream Media Chunk ➔ [spr-storage Engine]
                                                                     │
                                                                     ▼
                                                    [Shared Volume: spr-nosql-data]
                                                    Файл: data/scylladb_spr_emulation/rec_*.webm
```

### 📊 Диаграмма потоковой укладки видеокадров (Recording Stream Pipeline):
```mermaid
sequenceDiagram
    autonumber
    participant UI as 👑 Браузер Давида
    participant Proxy as 🌐 cloud-routing-proxy
    participant Storage as 🗄️ spr-storage
    participant Disk as 💾 NVMe Array (Volume)

    UI->>Proxy: HTTP POST /api/v1/records/upload?id=rec_178
    Proxy->>Storage: gRPC: StreamMediaChunk() (Открытие клиентского стрима)
    Storage->>Disk: Создание/открытие файла .webm, запись заголовка EBML (0x1A45DFA3)
    loop Побайтовая нарезка (64 Килобайта буфер)
        Proxy->>Storage: gRPC Stream Message: Send(MediaChunkRequest{Data: []byte})
        Storage->>Disk: Атомарная дисковая дозапись блока: os.Write() без аллокаций в куче
    end
    Proxy->>Storage: CloseAndRecv() (Запечатывание стрима)
    Storage->>Disk: sync & close дескриптора файла
    Storage-->>Proxy: MediaChunkResponse({Status: "SUCCESS"})
    Proxy-->>UI: HTTP 200 OK: UPLOAD_SUCCESS
```

---

## 🇺🇸 ENGLISH VERSION
The `spr-storage` distributed datastore engine (Port `:50060`) enables direct asynchronous recording of full-duplex session streams [2.1].

* **Zero-Copy Disk Commits**: Bypasses heavy JSON string structures by piping raw bytes arrays straight into system filesystem blocks via gRPC streams.
* **Accept-Ranges Continuity**: Serves archived containers back to the edge proxy cluster layer, enabling sub-millisecond video timeline seeking.
