FROM golang:1.24-alpine

# Установка необходимых пакетов
RUN apk add --no-cache git tzdata ca-certificates

# Создание рабочей директории
WORKDIR /app

# Копирование файлов проекта
COPY . .

# Загрузка зависимостей
RUN go mod download

# Сборка основного приложения и бота
RUN go build -o predictor cmd/main.go && \
    go build -o tgbot cmd/tgbot/main.go

# Определение переменной для выбора запуска
ENV APP_TYPE=app

# Скрипт запуска
CMD if [ "$APP_TYPE" = "bot" ]; then \
        /app/tgbot; \
    else \
        /app/predictor; \
    fi