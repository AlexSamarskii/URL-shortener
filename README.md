# Сервис сокращения ссылок (тестовое задание стажировка Ozon Банк)

url-shortener — сервис сокращения ссылок с выбором базы данных (memory|postgres) кэшированием Redis, rate limiting и фильтром Блума.

## Быстрый старт

### Требования

- Go 1.22+
- Docker и Docker Compose (для PostgreSQL, Redis и интеграционных тестов)

### Установка

```bash
git clone https://github.com/AlexSamarskii/URL-shortener.git
cd URL-shortener
```
### Конфигурация

- Все настройки задаются через переменные окружения (файл .env).
```bash
PORT=8080
ENABLE_METRICS=true
DOMAIN=http://localhost:8080
SHORT_CODE_LENGTH=10
STORAGE_TYPE=postgres # выбор типа хранилища (postgres|memory)
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=shortener
POSTGRES_PASSWORD=secret
POSTGRES_DB=shortener
POSTGRES_SSL_MODE=disable
POSTGRES_MAX_CONNS=10
POSTGRES_CONN_TIMEOUT_SEC=5
REDIS_ADDR=redis:6379
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_DIAL_TIMEOUT_SEC=5
REDIS_READ_TIMEOUT_SEC=3
REDIS_WRITE_TIMEOUT_SEC=3
RATE_LIMIT_MAX=100
RATE_LIMIT_WINDOW_SEC=60
RATE_LIMIT_SCRIPT_PATH=/scripts/rate_limit.lua
BLOOM_N=1000000
BLOOM_P=0.001
```

# Запуск сервиса

```bash
make run
```
### Запуск с Docker Compose
```bash
docker-compose up -d --build
```

## API Endpoints

### POST /shorten

Сокращает оригинальный URL.

**Тело запроса (application/json)**

| Поле         | Тип     | Обязательное | Описание                                                       |
|--------------|---------|--------------|----------------------------------------------------------------|
| `url`        | string  | да           | Оригинальный URL (схема http или https)                        |
| `expires_in` | integer | нет          | Время жизни в секундах (int опционально)                   |
| `alias`      | string  | нет          | Пользовательский короткий код (опционально ровно 10 символов, разрешены: `0-9A-Za-z_`) |

**Ответ (200 OK)**

| Поле         | Тип    | Описание                                           |
|--------------|--------|----------------------------------------------------|
| `short_code` | string | Сгенерированный или переданный короткий код        |
| `short_url`  | string | Полный короткий URL (домен + `/` + код)            |
| `expires_at` | string | ISO8601 timestamp или `null` (если без TTL)        |

**Коды ошибок**

| HTTP статус | Описание                           |
|-------------|------------------------------------|
| 400         | Некорректный URL или формат алиаса |
| 409         | Алиас уже существует               |
| 500         | Внутренняя ошибка сервера          |

**Пример**

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://career.ozon.ru/fintech/vacancy?id=131698788\u0026abt_att=1"}'
```

Response:
```json
{
  "short_code": "aB3dE5fG7h",
  "short_url": "http://localhost:8080/aB3dE5fG7h",
  "expires_at": null
}
```

## GET /{short_code}

Возвращает оригинальный URL.

**Ответ (200 OK)**

| HTTP статус | Описание                         |
|-------------|----------------------------------|     
| 404         | Короткий код не найден           |
| 410         | Срок действия ссылки истёк (Gone)|
| 500         | Внутренняя ошибка сервера        |

**Пример**

```bash
curl -v http://localhost:8080/aB3dE5fG7h
```
response:
```json
{"original_url":"https://career.ozon.ru/fintech/vacancy?id=131698788"}
```

## Метрики

Если `ENABLE_METRICS=true`, сервис предоставляет метрики Prometheus по эндпоинту `/metrics`.

### Доступные метрики

| Имя метрики                        | Тип       | Метки                          | Описание                                                       |
|------------------------------------|-----------|--------------------------------|----------------------------------------------------------------|
| `http_requests_total`              | Counter   | `method`, `endpoint`, `status` | Общее количество обработанных HTTP-запросов                    |
| `redirect_latency_seconds`         | Histogram | `cache_hit`                    | Длительность запросов перенаправления (в секундах). `cache_hit` = `"true"` или `"false"`. |
| `rate_limit_blocked_total`         | Counter   | `identifier`                   | Количество запросов, отклонённых ограничителем частоты. `identifier` – IP клиента. |
| `http_request_duration_seconds`   |  Histogram  |         `method`, `endpoint`  | Продолжительность одного http запроса        |   

### Пример

```bash
curl http://localhost:8080/metrics
```

## Тестирование

**Unit-тесты с покрытием**

процент покрытия 90.4%

```bash
make test-coverage
```
Все тесты (unit + integration)
```bash
make test-all
```

## Структура проекта
```text
├── cmd/
│   ├── migrate/           
│   └── service/ 
├── deployment/Dockerfile 
├── docs/                  # swagger docs  
├── configs/config.yaml     
├── internal/
│   ├── entity/dto         # Domain entities & errors
│   ├── handler/http/      # Gin handlers
│   ├── middleware/        # Rate limiting middleware
│   ├── pkg/
│   │   ├── config/        # Environment config loader
│   │   ├── logger/        # slog 
│   │   └── metrics/       # Prometheus metrics
│   ├── repository/
│   │   ├── memory/        # In-memory implementation 
│   │   ├── postgres/      # PostgreSQL implementation
│   │   └── mocks/         # Generated mocks
│   ├── usecase/           # Business logic
│   └── utils/
│       ├── bloom/         # Bloom filter 
│       ├── cache/         # Redis cache
│       └── rate_limiter/  # Redis token bucket
├── migrations/            # SQL migration files
├── scripts/rate_limit.lua # lua script for rate_limiting
├── Makefile
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## Development
- Генерация моков
```bash
make generate
```
- Линтер
```bash
make lint  
```
- Очистка
```bash
make clean
```
- Swagger
```bash
make swagger
```

---
## Замечания по производительности

- **Redis rate limiter** использует Lua-скрипт для атомарного ограничения token bucket, подходит для высоких нагрузок.
- **Фильтр Блума** снижает количество обращений в репозиторий для несуществующих коротких кодов.
- **Кэш Redis** хранит недавно запрошенные URL с TTL, производным от срока жизни оригинальной ссылки.
- **PostgreSQL** использует индексы на `short_code` и `original_url` для быстрого поиска.