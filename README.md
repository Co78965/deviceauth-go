# deviceauth-go

Go-библиотека клиентской аутентификации устройства. Встраивается в HTTP-сервер приложения: хранит Ed25519-ключи на машине процесса, выполняет challenge–response с сервисом DeviceAuth и отдаёт JSON.

Модуль: `github.com/Co78965/deviceauth-go`  
Пакет: `deviceauth`  
Go: `1.23.6+`

Браузерный клиент, который считает отпечаток и вызывает эти обработчики: [`deviceauth-js`](https://github.com/Co78965/deviceauth-js).

## Как это работает

1. Клиент передаёт **отпечаток устройства** (`fingerprint`) и, при регистрации, **идентификатор пользователя** (`user_id`).
2. Библиотека загружает или создаёт пару ключей Ed25519 для этого отпечатка.
3. Публичный ключ и отпечаток отправляются на удалённый сервис.
4. Сервис выдаёт `challenge`; клиент подписывает его приватным ключом и подтверждает подпись.
5. При успехе обработчик отвечает статусом `registered` или `authenticated`.

Приватный ключ **не уходит** на сервис. Подпись считается локально в процессе с этой библиотекой.

```
Клиент  →  RegisterHandler / AuthenticateHandler  →  DeviceAuth API
                ↓
         OS keyring или файлы в DEVICEAUTH_KEYS_DIR
```

## Установка

```bash
go get github.com/Co78965/deviceauth-go
```

Зависимости подтягиваются автоматически:

- `github.com/zalando/go-keyring` — системное хранилище секретов
- `github.com/joho/godotenv` — загрузка `.env` при инициализации

## Конфигурация

Задайте переменные окружения (или файл `.env` в рабочей директории процесса):

| Переменная | Обязательная | По умолчанию | Назначение |
|---|---|---|---|
| `DEVICEAUTH_SERVICE_URL` | да | пусто | Базовый URL сервиса, без завершающего `/` |
| `DEVICEAUTH_APP_ID` | нет | `default_app` | Идентификатор приложения на стороне сервиса |
| `DEVICEAUTH_KEYS_DIR` | нет | `./.deviceauth` | Каталог файлового хранилища ключей, если keyring недоступен |

Пример `.env`:

```env
DEVICEAUTH_SERVICE_URL=https://auth.example.com
DEVICEAUTH_APP_ID=my-web-app
DEVICEAUTH_KEYS_DIR=./.deviceauth
```

Таймаут HTTP-клиента к сервису фиксирован: **10 секунд**.

## Инициализация

Перед регистрацией маршрутов вызовите `Init`:

```go
ok := deviceauth.Init(false)
```

Аргумент `isEnvLoad`:

| Значение | Поведение |
|---|---|
| `false` | Сначала читается `.env` через `godotenv.Load()`. Если файла нет или чтение не удалось, `Init` возвращает `false`. |
| `true` | `.env` не загружается; используются уже заданные переменные окружения процесса. |

`Init` всегда читает `DEVICEAUTH_*` из окружения, выбирает хранилище ключей и возвращает `true` при успехе.

Порядок выбора хранилища:

1. **Системный keyring** (`service=deviceauth`, пользователь `default`) — Windows Credential Manager, macOS Keychain, Linux Secret Service / D-Bus.
2. Если keyring недоступен — **файлы** в `DEVICEAUTH_KEYS_DIR` (`0700` на каталог, `0600` на файлы ключей).

## Подключение к HTTP-серверу

Публичный API — два `http.HandlerFunc`. Их нужно смонтировать на свои пути и вызвать `Init` до обслуживания запросов.

```go
package main

import (
	"log"
	"net/http"

	"github.com/Co78965/deviceauth-go"
)

func main() {
	if !deviceauth.Init(false) {
		log.Fatal("deviceauth: не удалось инициализировать (проверьте .env и DEVICEAUTH_SERVICE_URL)")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/register", deviceauth.RegisterHandler())
	mux.HandleFunc("/auth/authenticate", deviceauth.AuthenticateHandler())

	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

Оба обработчика принимают только `POST` и тело `application/json`.

Пути `/auth/register` и `/auth/authenticate` — соглашение для [`deviceauth-js`](https://github.com/Co78965/deviceauth-js): фронт вызывает именно их. Для другого клиента пути можно выбрать любые.

### Регистрация устройства

`POST` на маршрут `RegisterHandler`.

**Тело запроса**

```json
{
  "fingerprint": "browser-or-device-fingerprint",
  "user_id": "user-123"
}
```

Оба поля обязательны.

**Успех** — `200 OK`

```json
{
  "status": "registered",
  "user_id": "user-123"
}
```

Внутри: загрузка/генерация ключей → `POST {SERVICE_URL}/api/devices/register` → подпись challenge → `POST {SERVICE_URL}/api/auth/verify` с `flow=register`.

### Аутентификация

`POST` на маршрут `AuthenticateHandler`.

**Тело запроса**

```json
{
  "fingerprint": "browser-or-device-fingerprint",
  "user_id": "user-123"
}
```

`fingerprint` обязателен. `user_id` необязателен: если передан, уходит в `POST /api/auth/challenge` и помогает сервису выбрать устройство, когда по отпечатку находится несколько записей.

**Успех** — `200 OK`

```json
{
  "status": "authenticated",
  "user_id": "<id с сервиса>"
}
```

Внутри: ключи по отпечатку → `POST {SERVICE_URL}/api/auth/challenge` → подпись → `POST {SERVICE_URL}/api/auth/verify` с `flow=auth`. `user_id` в ответе берётся из сервиса, не из запроса.

## Ошибки HTTP

Обработчики всегда отвечают JSON. Поле `error` — машинный код.

| HTTP | `error` | Когда |
|---|---|---|
| 405 | `method_not_allowed` | Метод не `POST` |
| 400 | `invalid_request` | Тело не JSON |
| 400 | `fingerprint_and_user_id_required` | Регистрация без `fingerprint` или `user_id` |
| 400 | `fingerprint_required` | Аутентификация без `fingerprint` |
| 409 | `multiple_devices` | Сервис вернул `error: multiple_devices` (несколько устройств; уточните `user_id`) |
| 401 | `auth_failed` | Остальные ошибки сервиса, сети, ключей, подписи |

Пример:

```json
{
  "error": "auth_failed"
}
```

Сообщения сервиса в поле `message` клиенту не проксируются — наружу уходит только код.

## Хранение ключей

Пара ключей привязана к **отпечатку**. При первом `LoadKeys` для неизвестного отпечатка генерируется новая Ed25519-пара.

**Keyring**

- Имя сервиса: `deviceauth`
- Имя записи: `default_keys_<fingerprint>`
- Значение: JSON `{"public_key":"<hex>","private_key":"<hex>"}`

**Файлы** (fallback)

- Путь: `{DEVICEAUTH_KEYS_DIR}/keys_<fingerprint>.json`
- Тот же JSON, права `0600`

Ключи и challenge/подпись кодируются **hex**. Подпись: `ed25519.Sign(privateKey, []byte(challenge))`.

Не коммитьте каталог `.deviceauth` и не копируйте файлы ключей между машинами, если это не задумано явно.

## Контракт с DeviceAuth API

Базовый URL — `DEVICEAUTH_SERVICE_URL`. Все вызовы — `POST`, `Content-Type: application/json`.

### `POST /api/devices/register`

Ожидаемый успех: **201 Created**.

```json
{
  "user_id": "<из запроса>",
  "app_id": "<DEVICEAUTH_APP_ID>",
  "fingerprint": "<отпечаток>",
  "public_key": "<hex Ed25519>"
}
```

Ответ:

```json
{
  "device_id": "...",
  "challenge": "..."
}
```

### `POST /api/auth/challenge`

Ожидаемый успех: **200 OK**.

```json
{
  "app_id": "<DEVICEAUTH_APP_ID>",
  "fingerprint": "<отпечаток>",
  "user_id": "<если передан>"
}
```

Ответ: `device_id`, `challenge`.

### `POST /api/auth/verify`

Ожидаемый успех: **200 OK**.

```json
{
  "device_id": "...",
  "signature": "<hex>",
  "flow": "register | auth"
}
```

Ответ: `status`, опционально `user_id` (используется при `flow=auth`).

Ошибка сервиса разбирается как JSON `{"error":"...","message":"..."}`. Код `multiple_devices` пробрасывается отдельно; любой другой код и неразбираемое тело становятся `auth_failed`.

## Публичный API пакета

Экспортируются только:

```go
func Init(isEnvLoad bool) bool
func RegisterHandler() http.HandlerFunc
func AuthenticateHandler() http.HandlerFunc
```

HTTP-клиент, хранилище и криптография — внутренние детали пакета.

## Типовые проблемы

**`Init` возвращает `false` при `Init(false)`**  
Нет `.env` в текущей рабочей директории. Положите файл рядом с процессом или вызывайте `Init(true)` после самостоятельной загрузки окружения.

**Пустой `DEVICEAUTH_SERVICE_URL`**  
`Init` всё равно может вернуть `true`. Запросы к сервису тогда падают, обработчики отвечают `auth_failed`.

**Keyring недоступен в CI / контейнере без D-Bus**  
Используется файловое хранилище. Задайте `DEVICEAUTH_KEYS_DIR` на том, который не попадает в git.

**`multiple_devices` при входе**  
Передайте тот же `user_id`, что при регистрации.

**Ключи «пропали» после смены `fingerprint`**  
Это ожидаемо: ключи индексируются отпечатком. Новый отпечаток — новая пара и повторная регистрация.

## Минимальный пример с уже загруженным окружением

```go
os.Setenv("DEVICEAUTH_SERVICE_URL", "https://auth.example.com")
os.Setenv("DEVICEAUTH_APP_ID", "my-web-app")

if !deviceauth.Init(true) {
    log.Fatal("init failed")
}

http.HandleFunc("/auth/register", deviceauth.RegisterHandler())
http.HandleFunc("/auth/authenticate", deviceauth.AuthenticateHandler())
```
