FROM golang:1.23-alpine AS builder

WORKDIR /app

# Установка необходимых пакетов
RUN apk update && apk add --no-cache gcc musl-dev

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код
COPY . .

# Сборка Telegram-бота
RUN CGO_ENABLED=1 GOOS=linux go build -a -o tgbot cmd/tgbot/main.go

# Сборка webhook-сервера
RUN CGO_ENABLED=1 GOOS=linux go build -a -o webhook cmd/stripe_webhook/main.go

# Финальный образ для Telegram-бота
FROM alpine:latest AS tgbot
RUN apk add --no-cache ca-certificates libc6-compat tzdata
WORKDIR /app
COPY --from=builder /app/tgbot .
COPY .env .
CMD ["./tgbot"]

# Финальный образ для webhook-сервера
FROM alpine:latest AS webhook
RUN apk add --no-cache ca-certificates libc6-compat tzdata
WORKDIR /app
COPY --from=builder /app/webhook .
COPY .env .
EXPOSE 8080
CMD ["./webhook"]