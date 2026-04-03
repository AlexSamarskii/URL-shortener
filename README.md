# Сервис сокращения ссылок (тестовое задание стажировка Ozon Банк)

Высокопроизводительный сервис сокращения ссылок с кэшированием на базе Redis, ограничением частоты запросов (rate limiting) и фильтром Блума, постоянным хранением в PostgreSQL и метриками Prometheus.

## Возможности

- Сокращение длинных URL с автоматической генерацией или пользовательским алиасом (ровно 10 символов, разрешены: `0-9A-Za-z_`).
- Опциональный TTL (время жизни) для коротких ссылок.
- Перенаправление на оригинальный URL с HTTP 301 (постоянное перенаправление).
- **Redis** для кэширования и ограничения частоты запросов (token bucket, скользящее окно с Lua-скриптом).
- **PostgreSQL** как основное хранилище с автоматическими миграциями.
- Фильтр Блума для быстрого отклонения несуществующих коротких кодов.
- Метрики Prometheus (эндпоинт `/metrics`).
- Graceful shutdown и структурированное логирование (`slog`).
- Интеграционные тесты с `testcontainers` (PostgreSQL, Redis).
- Makefile для типовых задач.

## Архитектура



## Технологический стек

| Компонент          | Технология                                                                   |
|--------------------|------------------------------------------------------------------------------|
| Язык               | Go 1.25                                                                      |
| Веб-фреймворк      | Gin                                                                          |
| База данных        | PostgreSQL 16+                                                               |
| Кэш и лимитер      | Redis 7+ (go‑redis/v9)                                                       |
| Фильтр Блума       | `bits-and-blooms/bloom/v3`                                                   |
| Миграции           | `golang-migrate/migrate/v4`                                                  |
| Метрики            | Prometheus client (`prometheus/client_golang`)                               |
| Тестирование       | `testify`, `testcontainers`, `gomock`                                        |
| Логирование        | `log/slog`                                                                   |

## Быстрый старт

### Требования

- Go 1.22+
- Docker и Docker Compose (для PostgreSQL, Redis и интеграционных тестов)

### Установка

```bash
git clone https://github.com/AlexSamarskii/URL-shortener.git
cd URL-shortener
make run
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
| `expires_in` | integer | нет          | Время жизни в секундах (положительное целое)                   |
| `alias`      | string  | нет          | Пользовательский короткий код (ровно 10 символов, разрешены: `0-9A-Za-z_`) |

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
  -d '{"url":"https://career.ozon.ru/fintech/vacancy?id=131698788"}'
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

Перенаправляет на оригинальный URL.

**Ответ**

| HTTP статус | Описание                         |
|-------------|----------------------------------|
| 301         | Постоянное перенаправление       |
| 404         | Короткий код не найден           |
| 410         | Срок действия ссылки истёк (Gone)|
| 500         | Внутренняя ошибка сервера        |

**Пример**

```bash
curl -v http://localhost:8080/aB3dE5fG7h
```

## Метрики

Если `ENABLE_METRICS=true`, сервис предоставляет метрики Prometheus по эндпоинту `/metrics`.

### Доступные метрики

| Имя метрики                        | Тип       | Метки                          | Описание                                                       |
|------------------------------------|-----------|--------------------------------|----------------------------------------------------------------|
| `http_requests_total`              | Counter   | `method`, `endpoint`, `status` | Общее количество обработанных HTTP-запросов                    |
| `redirect_latency_seconds`         | Histogram | `cache_hit`                    | Длительность запросов перенаправления (в секундах). `cache_hit` = `"true"` или `"false"`. |
| `rate_limit_blocked_total`         | Counter   | `identifier`                   | Количество запросов, отклонённых ограничителем частоты. `identifier` – IP клиента. |

### Пример

```bash
curl http://localhost:8080/metrics
```

## Тестирование

**Unit-тесты с покрытием**

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