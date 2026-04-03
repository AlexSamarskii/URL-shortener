FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /migrate ./cmd/migrate

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /server /app/
COPY --from=builder /migrate /app/

COPY configs/config.yaml /app/configs/config.yaml
COPY migrations /migrations

EXPOSE 8080

CMD ["/app/server"]