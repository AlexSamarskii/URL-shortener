# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o url_shortener ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем бинарник из builder
COPY --from=builder /app/url_shortener .

# Копируем конфиг (опционально, можно использовать переменные окружения)
COPY config.example.yaml ./config.yaml

EXPOSE 8080

CMD ["./url_shortener"]