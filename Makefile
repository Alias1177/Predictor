.PHONY: build run run-https build-docker run-docker stop-docker clean-certs generate-certs test-https

# Сборка проектов
build:
	go build -o bin/tgbot cmd/tgbot/main.go
	go build -o bin/webhook cmd/stripe_webhook/main.go
	go build -o bin/broadcast cmd/broadcast/main.go

# Запуск без HTTPS
run:
	USE_HTTPS=false ./bin/webhook

# Запуск с HTTPS и самоподписанным сертификатом
run-https:
	USE_HTTPS=true ./bin/webhook

# Запуск с HTTPS для IP сервера
run-https-ip:
	USE_HTTPS=true SERVER_IP=168.231.87.153 ./bin/webhook

# Запуск с HTTPS и Let's Encrypt (нужен домен)
run-https-letsencrypt:
	USE_HTTPS=true DOMAIN=your-domain.com ./bin/webhook

# Генерация самоподписанного сертификата
generate-certs:
	mkdir -p certs
	@if [ -n "$(SERVER_IP)" ]; then \
		echo "Генерируем сертификат для IP: $(SERVER_IP)"; \
		openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/C=RU/ST=Moscow/L=Moscow/O=Organization/CN=$(SERVER_IP)" -addext "subjectAltName=IP:$(SERVER_IP)"; \
	else \
		echo "Генерируем сертификат для localhost"; \
		openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/C=RU/ST=Moscow/L=Moscow/O=Organization/CN=localhost"; \
	fi

# Генерация сертификата для конкретного IP
generate-certs-for-ip:
	SERVER_IP=168.231.87.153 make generate-certs

# Генерация сертификата для IP
generate-certs-ip:
	mkdir -p certs
	@read -p "Введите IP адрес сервера: " ip; \
	echo "Генерируем сертификат для IP: $$ip"; \
	openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/C=RU/ST=Moscow/L=Moscow/O=Organization/CN=$$ip" -addext "subjectAltName=IP:$$ip"

# Тестирование HTTPS
test-https:
	./test-https.sh

test-https-ip:
	./test-https.sh 168.231.87.153 8080 8080

test-https-remote:
	@read -p "Введите домен для тестирования: " domain; \
	./test-https.sh $$domain

# Docker команды
build-docker:
	docker-compose build

run-docker:
	docker-compose up -d

run-docker-logs:
	docker-compose up

stop-docker:
	docker-compose down

restart-docker:
	docker-compose restart

# Очистка сертификатов
clean-certs:
	rm -rf certs/

# Установка зависимостей
deps:
	go mod tidy
	go mod download

# Тесты
test:
	go test ./...

# Рассылка сообщений
broadcast:
	./bin/broadcast

# Рассылка с предварительной сборкой
broadcast-run: build
	./bin/broadcast

# Очистка
clean:
	rm -rf bin/
	rm -rf certs/

# Установка всего
install: deps build generate-certs-for-ip

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  build              - Собрать проект"
	@echo "  run                - Запустить HTTP сервер"
	@echo "  run-https          - Запустить HTTPS сервер с самоподписанным сертификатом"
	@echo "  run-https-ip       - Запустить HTTPS сервер для IP 168.231.87.153"
	@echo "  run-https-letsencrypt - Запустить HTTPS сервер с Let's Encrypt"
	@echo "  generate-certs     - Генерировать самоподписанный сертификат"
	@echo "  generate-certs-for-ip - Генерировать сертификат для IP 168.231.87.153"
	@echo "  generate-certs-ip  - Генерировать сертификат для IP (интерактивно)"
	@echo "  test-https         - Тестировать HTTPS соединение (localhost)"
	@echo "  test-https-ip      - Тестировать HTTPS соединение для IP 168.231.87.153"
	@echo "  test-https-remote  - Тестировать HTTPS соединение (удаленный домен)"
	@echo "  build-docker       - Собрать Docker образы"
	@echo "  run-docker         - Запустить в Docker (detached)"
	@echo "  run-docker-logs    - Запустить в Docker с логами"
	@echo "  stop-docker        - Остановить Docker контейнеры"
	@echo "  clean-certs        - Удалить сертификаты"
	@echo "  deps               - Обновить зависимости"
	@echo "  clean              - Очистить собранные файлы"
	@echo "  broadcast          - Запустить рассылку сообщений"
	@echo "  broadcast-run      - Собрать и запустить рассылку"
	@echo "  install            - Полная установка (зависимости + сборка + сертификаты для IP)" 