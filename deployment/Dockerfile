FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /migrate ./cmd/migrate

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /server /app/
COPY --from=builder /migrate /app/

COPY .env /app/.env
COPY configs/config.yaml /app/configs/config.yaml
COPY scripts/rate_limit.lua /scripts/rate_limit.lua
COPY migrations /migrations

EXPOSE 8080

CMD ["/app/server"]