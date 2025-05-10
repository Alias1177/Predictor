# Forex Predictor Telegram Bot

A Telegram bot that provides forex predictions with premium subscription via Stripe payments.

## Features

- Telegram bot interface
- Currency pair selection
- Timeframe selection
- Premium subscription via Stripe
- Automated subscription expiration after 1 month
- Technical indicators and prediction
- PostgreSQL database for subscription storage

## Prerequisites

- Go 1.21 or higher
- PostgreSQL database
- Stripe account for payment processing
- Telegram Bot API token
- Twelve Data API key for forex data

## Installation

1. Clone the repository:

```bash
git clone <repository-url>
cd Predictor
```

2. Create a PostgreSQL database:

```bash
createdb predictor
```

3. Copy the example environment file and fill in your API keys:

```bash
cp .env.example .env
```

Edit the `.env` file with your actual keys:

- `TELEGRAM_BOT_TOKEN` - Get this from [BotFather](https://t.me/botfather)
- `TELEGRAM_BOT_USERNAME` - Your bot's username without the @ symbol
- `STRIPE_API_KEY` - Your Stripe API key from the Stripe dashboard
- `STRIPE_SUBSCRIPTION_PRICE_ID` - Create a price/product in Stripe and use its ID
- `STRIPE_WEBHOOK_SECRET` - Create a webhook endpoint in Stripe and use its signing secret
- `TWELVE_API_KEY` - Get this from [Twelve Data](https://twelvedata.com/)
- `DB_*` - PostgreSQL connection parameters

4. Build the bot:

```bash
go build -o tgbot cmd/tgbot/main.go
```

5. Build the webhook handler:

```bash
go build -o webhook cmd/stripe_webhook/main.go
```

## Running the Bot

1. Start the Telegram bot:

```bash
./tgbot
```

2. Start the webhook server (in a separate terminal):

```bash
./webhook
```

3. Expose your webhook server to the internet using a tool like ngrok:

```bash
ngrok http 8080
```

4. Configure your Stripe webhook to point to your public URL + /webhook path.

## Usage

1. Start a chat with your bot on Telegram using the `/start` command
2. Select a currency pair
3. Select a timeframe
4. Choose to run prediction (requires subscription)
5. Complete payment through Stripe
6. Access premium predictions for 1 month

## Docker Deployment

You can use the included Dockerfile and docker-compose.yml to run the services:

```bash
docker-compose up -d
```

## Project Structure

- `cmd/tgbot/`: Telegram bot implementation
- `cmd/stripe_webhook/`: Stripe webhook handler
- `internal/database/`: Database operations for subscriptions
- `internal/payment/`: Stripe payment integration
- `models/`: Data models and structures
- `config/`: Configuration management

## License

[MIT License](LICENSE)
