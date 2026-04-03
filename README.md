# URL Shortener Service (тестовое задание стажировка Ozon Банк)

High‑performance URL shortener with Redis‑based caching and rate limiting and bloom-filter, PostgreSQL persistence, and Prometheus metrics.

## Features

- Shorten long URLs with auto‑generated or custom alias (exactly 10 characters, allowed symbols: `0-9A-Za-z_`).
- Optional TTL (time‑to‑live) for short links.
- Redirect to original URL with HTTP 301 (permanent redirect).
- **Redis** for caching and token‑bucket rate limiting (sliding window with Lua script).
- **PostgreSQL** as primary storage with automatic migrations.
- Bloom filter to quickly reject non‑existent short codes.
- Prometheus metrics (`/metrics` endpoint).
- Graceful shutdown and structured logging (`slog`).
- Integration tests with `testcontainers` (PostgreSQL, Redis).
- Makefile for common tasks.

## Architecture



## Technology Stack

| Component       | Technology                                                                 |
|-----------------|----------------------------------------------------------------------------|
| Language        | Go 1.25                                                                    |
| Web framework   | Gin                                                                         |
| Database        | PostgreSQL 16+ (with `uuid-ossp` extension)                                |
| Cache & Limiter | Redis 7+ (go‑redis/v9)                                                     |
| Bloom filter    | `bits-and-blooms/bloom/v3`                                                 |
| Migrations      | `golang-migrate/migrate/v4`                                                |
| Metrics         | Prometheus client (`prometheus/client_golang`)                             |
| Testing         | `testify`, `testcontainers`, `gomock`                                      |
| Logging         | `log/slog` (JSON output)                                                   |

## Getting Started

### Prerequisites

- Go 1.22+
- Docker & Docker Compose (for PostgreSQL, Redis and integration tests)

### Installation

```bash
git clone https://github.com/AlexSamarskii/URL-shortener.git
cd URL-shortener
make run
```

### Configuration

- All settings are controlled via .env variables.
```bash
PORT=8080
ENABLE_METRICS=true
DOMAIN=http://localhost:8080
SHORT_CODE_LENGTH=10
STORAGE_TYPE=postgres
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

# Run service

```bash
make run
```
### Running with Docker Compose
```bash
docker-compose up -d --build
```


## API Endpoints

### POST /shorten

Shortens an original URL.

#### Request body (application/json)

| Field        | Type    | Required | Description                                      |
|--------------|---------|----------|--------------------------------------------------|
| `url`        | string  | yes      | Original URL (http or https scheme)             |
| `expires_in` | integer | no       | TTL in seconds (positive integer)               |
| `alias`      | string  | no       | Custom short code (exactly 10 characters, allowed: `0-9A-Za-z_`) |

#### Response (200 OK)

| Field         | Type   | Description                                   |
|---------------|--------|-----------------------------------------------|
| `short_code`  | string | Generated or provided short code              |
| `short_url`   | string | Full short URL (domain + `/` + code)          |
| `expires_at`  | string | ISO8601 timestamp or `null` (if no TTL)       |

#### Error responses

| HTTP status | Description                        |
|-------------|------------------------------------|
| 400         | Invalid URL or alias format        |
| 409         | Alias already exists               |
| 500         | Internal server error              |

Example:

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

### GET /{short_code}

Redirects to the original URL.

#### Response

| HTTP status | Description                         |
|-------------|-------------------------------------|
| 301         | Permanent redirect to original URL  |
| 404         | Short code not found                |
| 410         | URL expired (Gone)                  |
| 500         | Internal server error               |

#### Example

```bash
curl -v http://localhost:8080/aB3dE5fG7h
```

## Metrics

When `ENABLE_METRICS=true`, the service exposes Prometheus metrics at the `/metrics` endpoint.

### Available metrics

| Metric name                         | Type      | Labels                         | Description                                      |
|-------------------------------------|-----------|--------------------------------|--------------------------------------------------|
| `http_requests_total`               | Counter   | `method`, `endpoint`, `status` | Total number of HTTP requests processed         |
| `redirect_latency_seconds`          | Histogram | `cache_hit`                    | Duration of redirect requests (seconds). `cache_hit` is `"true"` or `"false"`. |
| `rate_limit_blocked_total`          | Counter   | `identifier`                   | Number of requests rejected by the rate limiter. `identifier` is usually the client IP. |



```bash
curl http://localhost:8080/metrics
```

## Testing

Unit Tests with Coverage
```bash
make test-coverage
```
All Tests (unit + integration)
```bash
make test-all
```

## Project Structure
```text
├── cmd/
│   ├── migrate/           
│   └── service/ 
├── deployment/Dockerfile   
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
- Generate Mocks
```bash
make generate
```
- Lint
```bash
make lint  
```
- Clean
```bash
make clean
```
---
### Performance Notes
- **Redis rate limiter** uses a Lua script for atomic token bucket limiting, suitable for high concurrency.
- **Bloom** filter reduces unnecessary repository lookups for non‑existent short codes.
- **Redis cache** stores recently accessed URLs with TTL derived from the original link’s expiration.
- PostgreSQL uses **indexes** on short_code and original_url for fast lookups.

