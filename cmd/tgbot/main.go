package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/Alias1177/Predictor/config"
	"github.com/Alias1177/Predictor/internal/analyze"
	"github.com/Alias1177/Predictor/internal/anomaly"
	"github.com/Alias1177/Predictor/internal/calculate"
	"github.com/Alias1177/Predictor/internal/database"
	"github.com/Alias1177/Predictor/internal/payment"
	"github.com/Alias1177/Predictor/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

var (
	supportedPairs = []string{
		"EUR/USD", "GBP/USD", "USD/JPY", "AUD/USD",
		"USD/CAD", "USD/CHF", "NZD/USD", "EUR/GBP",
		"EUR/JPY", "GBP/JPY", "AUD/CAD", "EUR/CAD",
	}

	supportedIntervals = []string{
		"1min", "5min", "15min", "30min", "1h", "4h", "1day",
	}

	// Map to store user states
	userStates = make(map[int64]*UserState)
)

// User state stages
const (
	StageInitial          = 0
	StageAwaitingPair     = 1
	StageAwaitingInterval = 2
	StageAwaitingPayment  = 3
	StagePremium          = 4
)

// UserState represents the current state of a user's interaction
type UserState struct {
	Stage        int       // 0: initial, 1: awaiting pair, 2: awaiting interval, 3: awaiting payment, 4: premium
	Symbol       string    // selected currency pair
	Interval     string    // selected time interval
	LastActivity time.Time // time of last activity
	PaymentURL   string    // Stripe payment URL
	SessionID    string    // Stripe session ID
}

// Global variables for database and payment service
var (
	db            *database.DB
	stripeService *payment.StripeService
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, relying on actual environment variables")
	}

	// Initialize database with PostgreSQL connection
	var err error
	dbParams := database.ConnectionParams{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}

	db, err = database.New(dbParams)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Stripe service
	stripeService = payment.NewStripeService()
}

func main() {
	// Setup logger
	lvl, _ := zerolog.ParseLevel("info")
	log.SetFlags(0)
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).Level(lvl).With().Timestamp().Logger()

	// Get bot token from environment
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		logger.Fatal().Msg("TELEGRAM_BOT_TOKEN not set in environment")
	}

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Telegram bot")
	}

	logger.Info().Str("username", bot.Self.UserName).Msg("Authorized on Telegram")

	// Setup update configuration
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	// Get updates channel
	updates := bot.GetUpdatesChan(updateConfig)

	// Start a goroutine to regularly check for expired subscriptions
	go checkExpiredSubscriptions()

	// Start handling updates
	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, update.Message, &logger)
		} else if update.CallbackQuery != nil {
			handleCallback(bot, update.CallbackQuery, &logger)
		}
	}
}

// checkExpiredSubscriptions runs periodically to update expired subscriptions
func checkExpiredSubscriptions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := db.CheckAndUpdateExpirations(); err != nil {
			log.Printf("Error checking expired subscriptions: %v", err)
		}
	}
}

// handleMessage processes incoming text messages
func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, logger *zerolog.Logger) {
	userID := message.From.ID
	chatID := message.Chat.ID

	// Check if this is a start command with parameters
	if strings.HasPrefix(message.Text, "/start") {
		parts := strings.Split(message.Text, " ")
		if len(parts) > 1 {
			param := parts[1]
			if param == "payment_success" {
				// User returned after successful payment
				handlePaymentSuccess(bot, userID, chatID)
				return
			} else if param == "payment_cancel" {
				// User cancelled payment
				handlePaymentCancel(bot, userID, chatID)
				return
			}
		}
	}

	// Get or initialize user state
	state, exists := userStates[userID]
	if !exists || message.Text == "/start" {
		userStates[userID] = &UserState{
			Stage:        StageInitial,
			LastActivity: time.Now(),
		}
		state = userStates[userID]

		// Check if user has an active subscription
		sub, err := db.GetSubscription(userID)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error retrieving subscription")
		} else if sub != nil && sub.Status == models.PaymentStatusAccepted {
			state.Stage = StagePremium
			state.Symbol = sub.CurrencyPair
			state.Interval = sub.Timeframe
		}

		// Send welcome message with main menu
		msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
		bot.Send(msg)
		return
	}

	// Update last activity
	state.LastActivity = time.Now()

	switch message.Text {
	case "/start", "Main Menu":
		msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
		bot.Send(msg)
		state.Stage = StageInitial
	case "Select Currency Pair":
		sendCurrencyPairMenu(bot, chatID)
		state.Stage = StageAwaitingPair
	case "Select Timeframe":
		if state.Symbol == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select a currency pair first.")
			bot.Send(msg)
			sendCurrencyPairMenu(bot, chatID)
			state.Stage = StageAwaitingPair
		} else {
			sendTimeframeMenu(bot, chatID)
			state.Stage = StageAwaitingInterval
		}
	case "Run Prediction":
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before running prediction.")
			bot.Send(msg)
			msg = tgbotapi.NewMessage(chatID, "What would you like to do?")
			msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
			bot.Send(msg)
			state.Stage = StageInitial
		} else {
			// Check subscription status
			sub, err := db.GetSubscription(userID)
			if err != nil {
				logger.Error().Err(err).Int64("user_id", userID).Msg("Error retrieving subscription")
				msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error. Please try again later.")
				bot.Send(msg)
				return
			}

			if sub == nil || sub.Status != models.PaymentStatusAccepted {
				// User needs to pay for subscription
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("To run predictions you need a premium subscription. The subscription costs $9.99 per month."))
				msg.ReplyMarkup = getPaymentKeyboard()
				bot.Send(msg)
				state.Stage = StageAwaitingPayment
			} else {
				// User has active subscription, run prediction
				runPrediction(bot, chatID, state, logger)
				// Update last predicted time
				if err := db.UpdateLastPredicted(userID); err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error updating last predicted time")
				}
			}
		}
	case "Subscribe Now":
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before subscribing.")
			bot.Send(msg)
			sendCurrencyPairMenu(bot, chatID)
			state.Stage = StageAwaitingPair
			return
		}

		// Create subscription in database
		_, err := db.CreateSubscription(userID, chatID, state.Symbol, state.Interval)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating subscription")
			msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error. Please try again later.")
			bot.Send(msg)
			return
		}

		// Create Stripe checkout session
		sessionID, paymentURL, err := stripeService.CreateCheckoutSession(userID, state.Symbol, state.Interval)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating Stripe session")
			msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error with the payment system. Please try again later.")
			bot.Send(msg)
			return
		}

		// Save payment info in user state
		state.PaymentURL = paymentURL
		state.SessionID = sessionID
		state.Stage = StageAwaitingPayment

		// Send payment instructions
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Please complete your payment to access premium predictions."))

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Pay Now", paymentURL),
			),
		)
		msg.ReplyMarkup = keyboard
		bot.Send(msg)

		// Add follow-up message
		followUp := tgbotapi.NewMessage(chatID, "After completing payment, return to this chat. Your subscription will be activated automatically.")
		bot.Send(followUp)
	case "/status":
		// Check subscription status
		sub, err := db.GetSubscription(userID)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error retrieving subscription")
			msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error. Please try again later.")
			bot.Send(msg)
			return
		}

		if sub == nil {
			msg := tgbotapi.NewMessage(chatID, "You don't have an active subscription. Select a currency pair and timeframe to subscribe.")
			bot.Send(msg)
		} else {
			var statusMsg string
			switch sub.Status {
			case models.PaymentStatusPending:
				statusMsg = "Your subscription is pending payment. Please complete the payment to activate your subscription."
			case models.PaymentStatusAccepted:
				daysLeft := int(sub.ExpiresAt.Sub(time.Now()).Hours() / 24)
				statusMsg = fmt.Sprintf("You have an active subscription for %s on %s timeframe. Your subscription will expire in %d days.", sub.CurrencyPair, sub.Timeframe, daysLeft)
			case models.PaymentStatusClosed:
				statusMsg = "Your subscription has expired. Please subscribe again to continue using premium features."
			default:
				statusMsg = "Your subscription status is unknown. Please contact support."
			}
			msg := tgbotapi.NewMessage(chatID, statusMsg)
			bot.Send(msg)
		}
	default:
		// Handle other inputs based on current stage
		switch state.Stage {
		case StageAwaitingPair: // Expecting currency pair
			if contains(supportedPairs, message.Text) {
				state.Symbol = message.Text
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected pair: %s\nNow select a timeframe.", message.Text))
				bot.Send(msg)
				sendTimeframeMenu(bot, chatID)
				state.Stage = StageAwaitingInterval
			} else {
				msg := tgbotapi.NewMessage(chatID, "Invalid currency pair. Please choose from the list:")
				bot.Send(msg)
				sendCurrencyPairMenu(bot, chatID)
			}
		case StageAwaitingInterval: // Expecting interval
			if contains(supportedIntervals, message.Text) {
				state.Interval = message.Text
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected timeframe: %s\nYou can now run the prediction.", message.Text))
				// Attach the main menu keyboard without sending welcome text
				msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
				bot.Send(msg)
				state.Stage = StageInitial
			} else {
				msg := tgbotapi.NewMessage(chatID, "Invalid timeframe. Please choose from the list:")
				bot.Send(msg)
				sendTimeframeMenu(bot, chatID)
			}
		default:
			msg := tgbotapi.NewMessage(chatID, "Please use the menu buttons to interact with the bot.")
			msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
			bot.Send(msg)
		}
	}
}

// handleCallback processes inline keyboard button presses
func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, logger *zerolog.Logger) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	data := callback.Data

	// Get or initialize user state
	state, exists := userStates[userID]
	if !exists {
		userStates[userID] = &UserState{
			Stage:        StageInitial,
			LastActivity: time.Now(),
		}
		state = userStates[userID]
	}

	// Update last activity
	state.LastActivity = time.Now()

	// Acknowledge the callback query
	bot.Request(tgbotapi.NewCallback(callback.ID, ""))

	if strings.HasPrefix(data, "pair_") {
		// Extract pair from callback data
		pair := strings.TrimPrefix(data, "pair_")
		state.Symbol = pair

		// Send confirmation message
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected pair: %s\nNow select a timeframe.", pair))
		bot.Send(msg)

		// Show timeframe selection
		sendTimeframeMenu(bot, chatID)
		state.Stage = StageAwaitingInterval
	} else if strings.HasPrefix(data, "interval_") {
		// Extract interval from callback data
		interval := strings.TrimPrefix(data, "interval_")
		state.Interval = interval

		// Send confirmation message with main menu keyboard but no welcome text
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected timeframe: %s\nYou can now run the prediction.", interval))
		msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
		bot.Send(msg)

		state.Stage = StageInitial
	} else if data == "run_prediction" {
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before running prediction.")
			bot.Send(msg)
			msg = tgbotapi.NewMessage(chatID, "What would you like to do?")
			msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
			bot.Send(msg)
			state.Stage = StageInitial
		} else {
			// Check subscription status
			sub, err := db.GetSubscription(userID)
			if err != nil {
				logger.Error().Err(err).Int64("user_id", userID).Msg("Error retrieving subscription")
				msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error. Please try again later.")
				bot.Send(msg)
				return
			}

			if sub == nil || sub.Status != models.PaymentStatusAccepted {
				// User needs to pay for subscription
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("To run predictions you need a premium subscription. The subscription costs $9.99 per month."))
				msg.ReplyMarkup = getPaymentKeyboard()
				bot.Send(msg)
				state.Stage = StageAwaitingPayment
			} else {
				// User has active subscription, run prediction
				runPrediction(bot, chatID, state, logger)
				// Update last predicted time
				if err := db.UpdateLastPredicted(userID); err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error updating last predicted time")
				}
			}
		}
	} else if data == "subscribe" {
		// Handle subscription
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before subscribing.")
			bot.Send(msg)
			sendCurrencyPairMenu(bot, chatID)
			state.Stage = StageAwaitingPair
			return
		}

		// Send a loading message
		loadingMsg := tgbotapi.NewMessage(chatID, "Creating payment session...")
		sentMsg, err := bot.Send(loadingMsg)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error sending loading message")
		}

		// Create subscription in database
		_, err = db.CreateSubscription(userID, chatID, state.Symbol, state.Interval)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating subscription")
			msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error. Please try again later.")
			bot.Send(msg)
			return
		}

		// Create Stripe checkout session
		sessionID, paymentURL, err := stripeService.CreateCheckoutSession(userID, state.Symbol, state.Interval)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating Stripe session")
			msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error with the payment system. Please try again later.")
			bot.Send(msg)
			return
		}

		// Save payment info in user state
		state.PaymentURL = paymentURL
		state.SessionID = sessionID
		state.Stage = StageAwaitingPayment

		// Edit the loading message to provide payment instructions
		editMsg := tgbotapi.NewEditMessageText(
			chatID,
			sentMsg.MessageID,
			fmt.Sprintf("Please complete your payment to access premium predictions."),
		)

		// Add payment URL button
		editMsg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("Pay Now", paymentURL),
				},
			},
		}

		bot.Send(editMsg)

		// Add follow-up message
		followUp := tgbotapi.NewMessage(chatID, "After completing payment, return to this chat. Your subscription will be activated automatically.")
		bot.Send(followUp)
	} else if data == "main_menu" {
		msg := tgbotapi.NewMessage(chatID, "What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard(state.Stage == StagePremium)
		bot.Send(msg)
		state.Stage = StageInitial
	}
}

// handlePaymentSuccess handles when a user returns after successful payment
func handlePaymentSuccess(bot *tgbotapi.BotAPI, userID, chatID int64) {
	// Check subscription status
	sub, err := db.GetSubscription(userID)
	if err != nil {
		log.Printf("Error retrieving subscription: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Sorry, there was an error. Please try again later.")
		bot.Send(msg)
		return
	}

	if sub == nil {
		msg := tgbotapi.NewMessage(chatID, "No subscription found. Please select a currency pair and timeframe to subscribe.")
		bot.Send(msg)
		return
	}

	// Check if we need to manually update the subscription status
	// This is a fallback if the webhook hasn't updated the status yet
	if sub.Status == models.PaymentStatusPending {
		log.Printf("Payment success callback received, but subscription status is still pending. Manually updating for user %d", userID)

		// Create a unique payment ID based on timestamp
		paymentID := fmt.Sprintf("manual_%d_%d", userID, time.Now().Unix())

		// Update subscription status directly
		if err := db.UpdateSubscriptionStatus(userID, models.PaymentStatusAccepted, paymentID); err != nil {
			log.Printf("Failed to manually update subscription status: %v", err)
			msg := tgbotapi.NewMessage(chatID, "Thank you for your payment!Your subscription is being processed and will be activated shortly. If it's not active in a few minutes, please contact support.")
			bot.Send(msg)
			return
		}

		// Reload subscription data after update
		sub, err = db.GetSubscription(userID)
		if err != nil {
			log.Printf("Error retrieving updated subscription: %v", err)
		}

		// Notify about manual activation
		msg := tgbotapi.NewMessage(chatID, "Thank you for your payment!\n Your subscription has been activated.\n"+
			"\n🚨Attention: Refusal of responsibility\n\nThe trading signals provided are intended exclusively for information and educational purposes. They are not an investment recommendation and cannot be considered as a financial council.\n\nThe user agrees that: \n• All trade decisions are made by him at his own risk. \n• He undertakes to use no more than 1-2% of the deposit per transaction. \n• He realizes that trade in financial markets is associated with a high level of risk. \n• The company is not responsible for the possible losses incurred as a result of the use of signals.\n\nUsing this service, you confirm that you are familiar with the risks and take full responsibility for your actions.\n\nHow to Read a Signal: Educational Guide\n\nEach signal you receive includes important data. Here's how to interpret it:\n\n⸻\n\n1. Direction\nThis shows whether the model expects the price to go UP (buy) or DOWN (sell).\nExample: Direction: DOWN means you may consider selling the asset.\n\n⸻\n\n2. Confidence\nThis indicates how strong the model's prediction is.\n • LOW: Less reliable, trade with caution.\n • MEDIUM: Moderate confidence.\n • HIGH: Strong signal with higher probability.\n\nYou should only trade when the confidence is HIGH or very close to it.\n\n⸻\n\n3. Score\nThis is a numerical value showing the strength and direction of the signal.\n • Positive scores (e.g., +5.0) = Buying pressure (BUY)\n • Negative scores (e.g., -5.0) = Selling pressure (SELL)\n • Values around 0 = Unclear direction, avoid trading.\n\nAs a rule of thumb:\n • Score > +4 → Strong Buy\n • Score < -4 → Strong Sell\n\n⸻\n\n4. Market Regime & Volatility\nThis tells you the current market conditions:\n • Trending: Prices are moving in one direction (up or down).\n • Ranging: Prices are moving sideways (no strong trend).\n • Volatility shows how active the market is. Higher volatility = more movement, more risk.\n\nUse this to decide whether it's a good time to enter a trade.\n\n⸻\n\n5. Indicators\nYou'll see technical indicators like RSI, MACD, Bollinger Bands, and more. These confirm the signal.\nFor example:\n • RSI under 50 = bearish pressure\n • MACD negative = bearish trend\n • Price near resistance = higher chance of reversal\n\n⸻\n\n6. Decision Factors\nThis section lists key technical signals the model uses to make its decision:\n • Patterns like Shooting Star, Double Top, Engulfing = Price reversal signals\n • Rejection at support/resistance zones\n • Indicator alignment (multiple indicators agreeing)\n\n⸻\n\n7. Trading Recommendations\nIncludes a sample trade setup:\n • Action: What to do (BUY/SELL)\n • Entry Price: Where to enter the trade\n • Stop Loss: Where to limit your loss\n • Take Profit: Where to close with a profit\n • Risk/Reward Ratio: Balance between risk and reward\n • Recommended Position Size: Based on 1% risk of your total capital\n\nYou're responsible for managing your risk.\nDo not trade emotionally or use high leverage. Stick to 1-2% risk per trade.\n\n⸻\n\nNote:\nThis is not financial advice. You trade at your own risk. For deeper knowledge, check out our PDF guide:\n\nhttps://t.me/Trade_Plus_Online_Bot")
		bot.Send(msg)
	} else if sub.Status == models.PaymentStatusAccepted {
		// Subscription is already active
		daysLeft := int(time.Until(sub.ExpiresAt).Hours() / 24)
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Your subscription is active! You can now run predictions for %s on %s timeframe. Your subscription will expire in %d days.", sub.CurrencyPair, sub.Timeframe, daysLeft))
		msg.ReplyMarkup = getMainMenuKeyboard(true)
		bot.Send(msg)
	} else {
		// Unexpected status
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Your subscription status is: %s. Please contact support if you believe this is an error.", sub.Status))
		bot.Send(msg)
	}

	// Update user state regardless of the status
	state, exists := userStates[userID]
	if exists {
		state.Stage = StagePremium
		state.Symbol = sub.CurrencyPair
		state.Interval = sub.Timeframe
	}
}

// handlePaymentCancel handles when a user returns after cancelling payment
func handlePaymentCancel(bot *tgbotapi.BotAPI, userID, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Your payment was cancelled. You can try again later when you're ready.")
	msg.ReplyMarkup = getMainMenuKeyboard(false)
	bot.Send(msg)

	// Update user state
	state, exists := userStates[userID]
	if exists {
		state.Stage = StageInitial
	}
}

// getMainMenuKeyboard returns the main menu keyboard
func getMainMenuKeyboard(isPremium bool) tgbotapi.ReplyKeyboardMarkup {
	if isPremium {
		return tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Select Currency Pair"),
				tgbotapi.NewKeyboardButton("Select Timeframe"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Run Prediction"),
			),
		)
	}

	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Select Currency Pair"),
			tgbotapi.NewKeyboardButton("Select Timeframe"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Run Prediction"),
		),
	)
}

// getPaymentKeyboard returns the keyboard for payment options
func getPaymentKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Subscribe ($9.99/month)", "subscribe"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back to Main Menu", "main_menu"),
		),
	)
}

// sendCurrencyPairMenu displays currency pair selection as inline buttons
func sendCurrencyPairMenu(bot *tgbotapi.BotAPI, chatID int64) {
	var keyboard [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, pair := range supportedPairs {
		// Create 2 pairs per row
		if i%2 == 0 && i > 0 {
			keyboard = append(keyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(pair, "pair_"+pair))
	}

	// Add the last row if it has any buttons
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}

	// Add a return to main menu button
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("← Back to Main Menu", "main_menu")})

	msg := tgbotapi.NewMessage(chatID, "Select a currency pair:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

// sendTimeframeMenu displays timeframe selection as inline buttons
func sendTimeframeMenu(bot *tgbotapi.BotAPI, chatID int64) {
	var keyboard [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, interval := range supportedIntervals {
		// Create 3 timeframes per row
		if i%3 == 0 && i > 0 {
			keyboard = append(keyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(interval, "interval_"+interval))
	}

	// Add the last row if it has any buttons
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}

	// Add a return to main menu button
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("← Back to Main Menu", "main_menu")})

	msg := tgbotapi.NewMessage(chatID, "Select a timeframe:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

// runPrediction executes the prediction with selected parameters
func runPrediction(bot *tgbotapi.BotAPI, chatID int64, state *UserState, logger *zerolog.Logger) {
	// Send processing message
	processingMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Running prediction for %s on %s timeframe...", state.Symbol, state.Interval))
	sentMsg, _ := bot.Send(processingMsg)

	// Create a config object with user selections and environment variables
	cfg := &models.Config{
		TwelveAPIKey:      os.Getenv("TWELVE_API_KEY"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		Symbol:            state.Symbol,
		Interval:          state.Interval,
		CandleCount:       getEnvInt("CANDLE_COUNT", 42),
		RSIPeriod:         getEnvInt("RSI_PERIOD", 11),
		MACDFastPeriod:    getEnvInt("MACD_FAST_PERIOD", 3),
		MACDSlowPeriod:    getEnvInt("MACD_SLOW_PERIOD", 11),
		MACDSignalPeriod:  getEnvInt("MACD_SIGNAL_PERIOD", 3),
		BBPeriod:          getEnvInt("BB_PERIOD", 19),
		BBStdDev:          getEnvFloat("BB_STD_DEV", 3.4),
		EMAPeriod:         getEnvInt("EMA_PERIOD", 7),
		ADXPeriod:         getEnvInt("ADX_PERIOD", 28),
		ATRPeriod:         getEnvInt("ATR_PERIOD", 10),
		RequestTimeout:    getEnvInt("REQUEST_TIMEOUT", 30),
		AdaptiveIndicator: getEnvBool("ADAPTIVE_INDICATOR", true),
		EnableBacktest:    false, // Disable backtesting for faster response
	}

	// Create client and context
	client := config.NewClient(cfg)
	ctx := context.Background()

	// Try to get candles
	candles, err := client.GetCandles(ctx)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Error fetching candles: %s", err.Error()))
		bot.Send(errMsg)
		return
	}

	// Calculate indicators
	indicators := calculate.CalculateAllIndicators(candles, cfg)

	// Multi-timeframe analysis (simplified)
	mtfData := map[string][]models.Candle{
		state.Interval: candles,
	}

	// Try to get multi-timeframe data if available
	moreData, err := calculate.GetMultiTimeframeData(ctx, cfg.TwelveAPIKey, cfg.Symbol)
	if err == nil && len(moreData) > 0 {
		for k, v := range moreData {
			mtfData[k] = v
		}
	}

	// Market regime and anomalies
	regime, err := anomaly.EnhancedMarketRegimeClassification(candles)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to classify market regime")
		regime = &models.MarketRegime{
			Type:             "UNKNOWN",
			Strength:         0,
			Direction:        "NEUTRAL",
			VolatilityLevel:  "NORMAL",
			MomentumStrength: 0,
			LiquidityRating:  "NORMAL",
			PriceStructure:   "UNKNOWN",
		}
	}
	anomalyData := anomaly.DetectMarketAnomalies(candles)

	// Generate prediction
	prediction, err := analyze.EnhancedPrediction(
		ctx,
		candles, indicators, mtfData, regime, anomalyData, cfg)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate prediction")
		errMsg := tgbotapi.NewMessage(chatID, "Error generating prediction. Please try again later.")
		bot.Send(errMsg)
		return
	}

	// Edit message to show loading status
	editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, "⏳ Analyzing market data...")
	bot.Send(editMsg)

	// Format the prediction message
	var resultText strings.Builder
	resultText.WriteString(fmt.Sprintf("*Prediction for %s (%s)*\n\n", state.Symbol, state.Interval))

	// Direction emoji
	directionEmoji := "⚖️"
	if prediction.Direction == "UP" {
		directionEmoji = "🔼"
	} else if prediction.Direction == "DOWN" {
		directionEmoji = "🔽"
	}

	resultText.WriteString(fmt.Sprintf("*Direction:* %s %s\n", directionEmoji, prediction.Direction))
	resultText.WriteString(fmt.Sprintf("*Confidence:* %s\n", prediction.Confidence))
	resultText.WriteString(fmt.Sprintf("*Score:* %.2f\n\n", prediction.Score))

	// Market regime
	resultText.WriteString(fmt.Sprintf("*Market Regime:* %s (%s)\n", regime.Type, regime.Direction))
	resultText.WriteString(fmt.Sprintf("*Regime Strength:* %.2f\n", regime.Strength))
	resultText.WriteString(fmt.Sprintf("*Volatility:* %s\n\n", regime.VolatilityLevel))

	// Key Indicators
	resultText.WriteString("*Key Indicators:*\n")
	resultText.WriteString(fmt.Sprintf("RSI: %.2f | ", indicators.RSI))
	resultText.WriteString(fmt.Sprintf("MACD: %.5f\n", indicators.MACD))
	resultText.WriteString(fmt.Sprintf("BB: %.5f / %.5f / %.5f\n", indicators.BBLower, indicators.BBMiddle, indicators.BBUpper))
	resultText.WriteString(fmt.Sprintf("ADX: %.2f | ", indicators.ADX))
	resultText.WriteString(fmt.Sprintf("ATR: %.5f\n", indicators.ATR))

	// Factors
	resultText.WriteString("\n*Decision Factors:*\n")
	for i, factor := range prediction.Factors {
		resultText.WriteString(fmt.Sprintf("%d. %s\n", i+1, factor))
	}

	// Price data
	if len(candles) > 0 {
		currentPrice := candles[len(candles)-1].Close
		resultText.WriteString(fmt.Sprintf("\n*Current Price:* %.5f\n", currentPrice))
	}

	if prediction.TradingSuggestion != nil && prediction.TradingSuggestion.Action != "NO_TRADE" {
		resultText.WriteString("\n\n*Trading Recommendations:*\n")
		resultText.WriteString(fmt.Sprintf("Action: %s\n", prediction.TradingSuggestion.Action))
		resultText.WriteString(fmt.Sprintf("Entry Price: %.5f\n", prediction.TradingSuggestion.EntryPrice))
		resultText.WriteString(fmt.Sprintf("Stop Loss: %.5f\n", prediction.TradingSuggestion.StopLoss))
		resultText.WriteString(fmt.Sprintf("Take Profit: %.5f\n", prediction.TradingSuggestion.TakeProfit))
		resultText.WriteString(fmt.Sprintf("Risk/Reward Ratio: %.1f\n", prediction.TradingSuggestion.RiskRewardRatio))
		resultText.WriteString(fmt.Sprintf("Recommended Position Size: %.2f\n", prediction.TradingSuggestion.PositionSize))
		resultText.WriteString(fmt.Sprintf("Risk per Trade: %.1f%%\n", prediction.TradingSuggestion.AccountRisk))
	}

	// Send the final result
	resultMsg := tgbotapi.NewMessage(chatID, resultText.String())
	resultMsg.ParseMode = "Markdown"
	bot.Send(resultMsg)

	//// Try to get prediction from OpenAI if API key is available
	//if cfg.OpenAIAPIKey != "" {
	//	aiPrompt := gpt.FormatPrompt(candles, cfg.Symbol)
	//	aiMsg := tgbotapi.NewMessage(chatID, "Getting AI analysis...")
	//	sentAiMsg, _ := bot.Send(aiMsg)
	//
	//	// This is asynchronous so user doesn't have to wait
	//	go func() {
	//		// Create a buffer to capture GPT's output
	//		aiOutput := captureGPTOutput(cfg.OpenAIAPIKey, aiPrompt)
	//
	//		// Update the message with AI prediction
	//		editAiMsg := tgbotapi.NewEditMessageText(
	//			chatID,
	//			sentAiMsg.MessageID,
	//			fmt.Sprintf("*AI Analysis:*\n\n%s", aiOutput),
	//		)
	//		editAiMsg.ParseMode = "Markdown"
	//		bot.Send(editAiMsg)
	//	}()
	//}
}

// Helper function to get integer environment variables
func getEnvInt(key string, defaultVal int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultVal
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultVal
	}
	return value
}

// Helper function to get float environment variables
func getEnvFloat(key string, defaultVal float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultVal
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultVal
	}
	return value
}

// Helper function to get boolean environment variables
func getEnvBool(key string, defaultVal bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultVal
	}
	return valueStr == "true" || valueStr == "1" || valueStr == "yes"
}

// captureGPTOutput captures the output from the OpenAI API
//func captureGPTOutput(apiKey, prompt string) string {
//	// This is a simple wrapper around the gpt package that captures the output
//	client := &gptClient{}
//	return client.askGPT(apiKey, prompt)
//}

// gptClient is a simple wrapper for gpt.AskGPT that captures the output
//type gptClient struct{}

//func (g *gptClient) askGPT(apiKey, prompt string) string {
//	// Adapted from your gpt.AskGPT function to return the result as a string
//	// instead of printing it to the console
//	ctx := context.Background()
//	result, err := gpt.ProcessGPT(ctx, apiKey, prompt)
//	if err != nil {
//		return "Error getting AI prediction: " + err.Error()
//	}
//	return result
//}

// contains checks if a string exists in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
