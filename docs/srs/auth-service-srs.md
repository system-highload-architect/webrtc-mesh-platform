# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): AUTH SERVICE

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Назначение и контур криптографической защиты
Микросервис `auth-service` реализует изолированный периметр безопасности (Security Plane) и отвечает за сквозную проверку b2b PKI-сертификатов сотрудников компании [2.1].

### 2. Системные требования и крипто-протоколы
* **Спецификация gRPC-интерфейса**: Сервис обязан принимать запросы исключительно по бинарному протоколу gRPC. Текстовые или небезопасные REST-соединения в данном контуре запрещены.
* **Валидация X.509**: Компонент обязан разбирать сырые байты сертификатов (DER-кодировка), проверять криптографическую цепочку электронных подписей до доверенного Root CA и сверять серийные номера со списками отзыва сертификатов (CRL).
* **Генерация Ролевой Модели (JWT)**: При успешном проходе PKI-досмотра, сервис обязан сгенерировать криптографически подписанный токен JWT. В тело токена (Claims) обязаны зашиваться b2b-роли пользователя: `ORGANIZER` (Модератор Давид, полный доступ к админ-панели оркестрации) или `GUEST` (Рядовой сотрудник, пассивные медиа-права) [2.1].
* **SLA на авторизацию**: Время выполнения крипто-проверки одного сертификата не должно превышать **45 миллисекунд** при пиковой нагрузке.

---

## 🇺🇸 ENGLISH VERSION

### 1. Functional Scope & Security Boundaries
The `auth-service` handles identity verification across the ecosystem, exposing a single secure gRPC interface to authorize incoming participant roles [2.1].

### 2. Cryptographic Compliance Requirements
* **PKI Assertions**: Must execute low-level parsing of raw X.509 DER certificates, checking active CRL lists to block compromised or revoked corporate signatures.
* **JWT Token Issuance**: Successfully checked entities must receive a symetrically signed JSON Web Token carrying hardened RBAC metadata properties (`ORGANIZER` vs `GUEST` claims) [2.1].
* **Latency tail**: The execution window for heavy cryptographic signature validations is strictly bound to $\le 45\text{ms}$.
