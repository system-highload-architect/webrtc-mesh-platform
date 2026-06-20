# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): CORE PKG ALGORITHMS

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Назначение и Требования к Сложности Алгоритмов
Пакет `internal/pkg` инкапсулирует базовые математические структуры данных общего назначения, на которых зиждется производительность всей платформы.

### 2. Жесткие Критерии Эффективности ОЗУ и CPU
* **Реактивный LRU-Кэш**: Обязан гарантировать временную сложность чтения и вытеснения комнат строго за **$O(1)$** [1.1]. Максимальная емкость кэша запечатывается на уровне **1000 элементов**. Превышение лимита должно триггерить вытеснение хвостового узла Double Linked List [2.1].
* **Префиксное дерево Trie**: Сложность поиска автодополнения слова обязана составлять строго **$O(K)$**, где $K$ — длина вводимого пользователем префикса [1.1]. Потребление памяти дерева должно быть минимизировано за счет разреженного хранения рун алфавита.
* **Битовое Колесо Времени**: Метод `.Tick()` обязан выполнять побитовое сканирование маски `uint64` за **1 такт CPU** без аллокаций в куче ($O(1)$) [1.1]. Ложные срабатывания должны исключаться глубоким стиранием ключей из мап корзин `tw.buckets` [1.1].

---

## 🇺🇸 ENGLISH VERSION

### 1. Core Algorithmic Boundaries
The `internal/pkg` shared framework isolates low-level algorithms from service-layer state domains to guarantee bounded memory execution tracks.

### 2. Algorithmic Complexity Checkpoints
* **Reactive LRU Constraints**: Lookup and eviction must complete inside an strict **$O(1)$** timeline boundary [1.1].
* **Trie Auto-Prediction Bounds**: Search runtime is fixed at **$O(K)$** where $K$ defines input prefix byte size, rendering string length independent of total dictionary node scale [1.1].
* **Bitmapped Wheel Limits**: Tick cycles must run as a zero-allocation operation evaluating bit masks within **1 single CPU clock cycle** [1.1].
