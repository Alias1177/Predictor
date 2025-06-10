# Forex Predictor Telegram Bot

A Telegram bot that provides forex predictions with premium subscription via Stripe payments.

## Features

- Telegram bot interface
- Currency pair selection
- Timeframe selection
- Premium subscription via Stripe
- **Subscription cancellation functionality**
- Automated subscription expiration after 1 month
- Technical indicators and prediction
- PostgreSQL database for subscription storage
- **HTTPS support with Let's Encrypt or self-signed certificates**

## Prerequisites

- Go 1.21 or higher
- PostgreSQL database
- Stripe account for payment processing
- Telegram Bot API token
- Twelve Data API key for forex data
- **OpenSSL (для генерации сертификатов)**

## Quick Start

```bash
# Клонирование и установка
git clone <repository-url>
cd Predictor
make install

# Настройка переменных окружения
cp env.example .env
# Отредактируй .env файл

# Запуск с HTTPS
make run-https
```

## HTTPS Configuration

### Вариант 1: Let's Encrypt (рекомендуется для продакшена)

```bash
# В .env файле:
USE_HTTPS=true
DOMAIN=yourdomain.com

# Запуск
make run-https-letsencrypt
```

### Вариант 2: Самоподписанный сертификат (для тестирования)

```bash
# Генерация сертификата
make generate-certs

# В .env файле:
USE_HTTPS=true

# Запуск
make run-https
```

### Вариант 3: Собственные сертификаты

```bash
# В .env файле:
USE_HTTPS=true
TLS_CERT_FILE=/path/to/your/cert.pem
TLS_KEY_FILE=/path/to/your/key.pem

# Запуск
make run-https
```

## Installation Details

1. Clone the repository:

```bash
git clone <repository-url>
cd Predictor
```

2. Install dependencies and build:

```bash
make install
```

3. Create a PostgreSQL database:

```bash
createdb predictor
```

4. Copy the example environment file and fill in your API keys:

```bash
cp env.example .env
```

Edit the `.env` file with your actual keys:

**Basic Configuration:**
- `TELEGRAM_BOT_TOKEN` - Get this from [BotFather](https://t.me/botfather)
- `TELEGRAM_BOT_USERNAME` - Your bot's username without the @ symbol
- `STRIPE_API_KEY` - Your Stripe API key from the Stripe dashboard
- `STRIPE_SUBSCRIPTION_PRICE_ID` - Create a price/product in Stripe and use its ID
- `STRIPE_WEBHOOK_SECRET` - Create a webhook endpoint in Stripe and use its signing secret
- `TWELVE_API_KEY` - Get this from [Twelve Data](https://twelvedata.com/)
- `DB_*` - PostgreSQL connection parameters

**HTTPS Configuration:**
- `USE_HTTPS` - Set to `true` to enable HTTPS
- `DOMAIN` - Your domain for Let's Encrypt (optional)
- `CERT_DIR` - Directory for certificates (default: `./certs`)
- `TLS_CERT_FILE` - Path to certificate file
- `TLS_KEY_FILE` - Path to private key file

## Running the Bot

### Local Development

```bash
# Build the projects
make build

# Start the Telegram bot
./bin/tgbot

# Start the webhook server (HTTP)
make run

# Start the webhook server (HTTPS)
make run-https
```

### Docker Deployment

```bash
# Build and run with Docker
make build-docker
make run-docker

# Or with logs
make run-docker-logs
```

## Stripe Webhook Configuration

### For HTTPS with Let's Encrypt:
1. Set your domain in `.env`: `DOMAIN=yourdomain.com`
2. Point your domain A-record to your server IP
3. Start the server: `make run-https-letsencrypt`
4. Configure Stripe webhook URL: `https://yourdomain.com/webhook`

### For HTTPS with self-signed certificate:
1. Generate certificate: `make generate-certs`
2. Start server: `make run-https`
3. Use ngrok for public access: `ngrok http 8080 --scheme=https`
4. Configure Stripe webhook URL: `https://your-ngrok-domain.ngrok.io/webhook`

### For HTTP (development only):
1. Start server: `make run`
2. Use ngrok: `ngrok http 8080`
3. Configure Stripe webhook URL: `https://your-ngrok-domain.ngrok.io/webhook`

## Available Commands

```bash
make help  # Показать все доступные команды
```

## Security Notes

- **Never use self-signed certificates in production**
- Let's Encrypt certificates are automatically renewed
- Keep your private keys secure
- Use environment variables for sensitive data

## Project Structure

- `cmd/tgbot/`: Telegram bot implementation
- `cmd/stripe_webhook/`: Stripe webhook handler with HTTPS support
- `internal/database/`: Database operations for subscriptions
- `internal/payment/`: Stripe payment integration
- `models/`: Data models and structures
- `config/`: Configuration management
- `certs/`: Directory for SSL certificates (auto-created)

## Troubleshooting

### Certificate Issues
```bash
# Regenerate self-signed certificate
make clean-certs
make generate-certs

# Check certificate validity
openssl x509 -in certs/server.crt -text -noout
```

### Let's Encrypt Issues
- Ensure your domain points to your server
- Port 80 must be accessible for ACME challenge
- Check DNS propagation: `nslookup yourdomain.com`

### Docker Issues
```bash
# View logs
docker-compose logs webhook

# Restart services
make restart-docker
```

## License

[MIT License](LICENSE)
