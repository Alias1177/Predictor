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
		"XBR/USD", "XAU/USD", "XAG/USD",
		// Криптовалютные пары
		"ETH/USD", "SOL/USD", "XRP/USD", "ADA/USD",
		"AAVE/USD", "BNB/USD", "DOT/USD", "BTC/USD",
	}

	supportedIntervals = []string{
		"1min", "5min", "15min", "30min", "1h", "4h", "1day",
	}

	// Промокод для бесплатного доступа - МЕНЯЙ ЗДЕСЬ НА СВОЙ
	FREE_PROMO_CODE   = "FREEACCESS2025"
	FREE24_PROMO_CODE = "FREE24"
	TEST_PROMO_CODE   = "TEST" // Промокод для 30-дневного бесплатного доступа

	// Map to store user states
	userStates = make(map[int64]*UserState)

	// Даты истечения промокодов
	promoExpirationDates = map[string]time.Time{
		"FREEACCESS2025": time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		"FREE24":         time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
		"TEST":           time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
	}
)

// User state stages
const (
	StageInitial           = 0
	StageAwaitingPair      = 1
	StageAwaitingInterval  = 2
	StageAwaitingFreePromo = 3
	StageAwaitingPayment   = 4
	StagePremium           = 5
)

// UserState represents the current state of a user's interaction
type UserState struct {
	Stage        int       // 0: initial, 1: awaiting pair, 2: awaiting interval, 3: awaiting free promo, 4: awaiting payment, 5: premium
	Symbol       string    // selected currency pair
	Interval     string    // selected time interval
	LastActivity time.Time // time of last activity
	PaymentURL   string    // Stripe payment URL
	SessionID    string    // Stripe session ID
	PromoCode    string    // Current promo code being used
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

	// Запускаем таймеры для каждого промокода
	for promoCode, expirationDate := range promoExpirationDates {
		go schedulePromoNotification(bot, promoCode, expirationDate, &logger)
	}

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
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes for better precision
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
		msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
		bot.Send(msg)
		return
	}

	// Update last activity
	state.LastActivity = time.Now()

	switch message.Text {
	case "/start", "Main Menu":
		msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
		bot.Send(msg)
		state.Stage = StageInitial
	case "📊 Select Pair & Timeframe":
		sendCurrencyPairMenu(bot, chatID)
		state.Stage = StageAwaitingPair
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
	case "🔮 Run Prediction", "Run Prediction":
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before running prediction.")
			bot.Send(msg)
			msg = tgbotapi.NewMessage(chatID, "What would you like to do?")
			msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
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
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("To run predictions you need a premium subscription. The subscription costs $4.99 per month."))
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
	case "💎 Subscribe", "Subscribe Now":
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before subscribing.")
			bot.Send(msg)
			sendCurrencyPairMenu(bot, chatID)
			state.Stage = StageAwaitingPair
			return
		}

		// Go directly to payment
		proceedToPayment(bot, userID, chatID, state, logger)
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
	case "/config":
		// Debug command to check configuration (should be removed in production)
		if err := stripeService.ValidateConfig(); err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Configuration error: %v", err))
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "✅ Payment system configuration is valid")
			bot.Send(msg)
		}
	case "/check_sub":
		// Проверить подписку с детальной информацией
		sub, err := db.GetSubscription(userID)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Database error: %v", err))
			bot.Send(msg)
			return
		}

		if sub == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ No subscription found in database")
			bot.Send(msg)
			return
		}

		timeLeft := sub.ExpiresAt.Sub(time.Now())
		var timeLeftStr string
		if timeLeft > 0 {
			days := int(timeLeft.Hours() / 24)
			hours := int(timeLeft.Hours()) % 24
			timeLeftStr = fmt.Sprintf("%d days, %d hours", days, hours)
		} else {
			timeLeftStr = "EXPIRED"
		}

		msg := fmt.Sprintf(
			"📊 Subscription Details:\n"+
				"Status: %s\n"+
				"Created: %s\n"+
				"Expires: %s\n"+
				"Time left: %s\n"+
				"Current time: %s\n"+
				"Currency: %s\n"+
				"Timeframe: %s",
			sub.Status,
			sub.CreatedAt.Format("2006-01-02 15:04:05"),
			sub.ExpiresAt.Format("2006-01-02 15:04:05"),
			timeLeftStr,
			time.Now().Format("2006-01-02 15:04:05"),
			sub.CurrencyPair,
			sub.Timeframe,
		)

		respMsg := tgbotapi.NewMessage(chatID, msg)
		bot.Send(respMsg)
	case "/debug":
		// Debug command to check subscription data
		sub, err := db.GetSubscription(userID)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Database error: %v", err))
			bot.Send(msg)
			return
		}

		if sub == nil {
			msg := tgbotapi.NewMessage(chatID, "No subscription found in database")
			bot.Send(msg)
			return
		}

		debugInfo := fmt.Sprintf(`📋 Subscription Debug Info:
User ID: %d
Status: %s
Created: %s
Expires: %s
Payment ID: %s
Stripe Sub ID: %s
Currency Pair: %s
Timeframe: %s`,
			sub.UserID,
			sub.Status,
			sub.CreatedAt.Format("2006-01-02 15:04:05"),
			sub.ExpiresAt.Format("2006-01-02 15:04:05"),
			sub.PaymentID,
			sub.StripeSubscriptionID,
			sub.CurrencyPair,
			sub.Timeframe)

		msg := tgbotapi.NewMessage(chatID, debugInfo)
		bot.Send(msg)
	case "/fix":
		// Command to find and link missing Stripe subscription
		sub, err := db.GetSubscription(userID)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Database error: %v", err))
			bot.Send(msg)
			return
		}

		if sub == nil {
			msg := tgbotapi.NewMessage(chatID, "No subscription found in database")
			bot.Send(msg)
			return
		}

		if sub.StripeSubscriptionID != "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Stripe subscription already linked: %s", sub.StripeSubscriptionID))
			bot.Send(msg)
			return
		}

		// Send searching message
		searchMsg := tgbotapi.NewMessage(chatID, "🔍 Searching for your subscription in Stripe...")
		sentMsg, _ := bot.Send(searchMsg)

		// Calculate search timeframe (subscription created date - 1 hour)
		searchAfter := sub.CreatedAt.Unix() - 3600 // 1 hour before subscription creation

		// Try advanced search with creation time
		stripeSubscription, err := stripeService.FindSubscriptionAdvanced(userID, searchAfter)
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fmt.Sprintf("❌ Could not find subscription: %v\n\nTry manual search in Stripe Dashboard:\nhttps://dashboard.stripe.com/subscriptions\n\nLook for subscription created around: %s", err, sub.CreatedAt.Format("2006-01-02 15:04:05")))
			bot.Send(editMsg)
			return
		}

		// Update database with found subscription ID
		if err := db.UpdateStripeSubscriptionID(userID, stripeSubscription.ID); err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fmt.Sprintf("❌ Failed to update database: %v", err))
			bot.Send(editMsg)
			return
		}

		// Success message
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fmt.Sprintf("✅ Found and linked subscription!\nStripe ID: %s\nCreated: %s\n\nNow you can use 'Cancel Subscription' button.", stripeSubscription.ID, time.Unix(stripeSubscription.Created, 0).Format("2006-01-02 15:04:05")))
		bot.Send(editMsg)
	case "/test_cancel":
		// Test command to check cancellation process without actually cancelling
		sub, err := db.GetSubscription(userID)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Database error: %v", err))
			bot.Send(msg)
			return
		}

		if sub == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ No subscription found in database")
			bot.Send(msg)
			return
		}

		testMsg := fmt.Sprintf("🔍 Subscription Test Info:\n\n📊 Database:\n• User ID: %d\n• Status: %s\n• Created: %s\n• Stripe ID: %s\n\n🔍 Stripe Search Test:",
			sub.UserID,
			sub.Status,
			sub.CreatedAt.Format("2006-01-02 15:04:05"),
			sub.StripeSubscriptionID)

		if sub.StripeSubscriptionID != "" {
			testMsg += fmt.Sprintf("\n• Has Stripe ID: ✅\n• ID: %s", sub.StripeSubscriptionID)
		} else {
			testMsg += "\n• Missing Stripe ID: ⚠️"
			// Try to find subscription
			if stripeSubscription, err := stripeService.FindSubscriptionByUserID(userID); err == nil {
				testMsg += fmt.Sprintf("\n• Found by search: ✅\n• Found ID: %s\n• Status: %s", stripeSubscription.ID, stripeSubscription.Status)
			} else {
				testMsg += fmt.Sprintf("\n• Search failed: ❌\n• Error: %v", err)
			}
		}

		msg := tgbotapi.NewMessage(chatID, testMsg)
		bot.Send(msg)
	case "/force_cancel":
		// Force cancel subscription if we can find it
		sub, err := db.GetSubscription(userID)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Database error: %v", err))
			bot.Send(msg)
			return
		}

		if sub == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ No subscription found in database")
			bot.Send(msg)
			return
		}

		if sub.Status == models.PaymentStatusClosed {
			msg := tgbotapi.NewMessage(chatID, "ℹ️ Subscription already cancelled in database")
			bot.Send(msg)
			return
		}

		// Try to find and cancel subscription
		var cancelled []string
		var errors []string

		// Try with existing ID
		if sub.StripeSubscriptionID != "" {
			if err := stripeService.CancelSubscription(sub.StripeSubscriptionID); err != nil {
				errors = append(errors, fmt.Sprintf("ID %s: %v", sub.StripeSubscriptionID, err))
			} else {
				cancelled = append(cancelled, sub.StripeSubscriptionID)
			}
		}

		// Try to find more subscriptions
		if stripeSubscription, err := stripeService.FindSubscriptionByUserID(userID); err == nil {
			if stripeSubscription.ID != sub.StripeSubscriptionID {
				if err := stripeService.CancelSubscription(stripeSubscription.ID); err != nil {
					errors = append(errors, fmt.Sprintf("ID %s: %v", stripeSubscription.ID, err))
				} else {
					cancelled = append(cancelled, stripeSubscription.ID)
					// Update database
					db.UpdateStripeSubscriptionID(userID, stripeSubscription.ID)
				}
			}
		}

		// Cancel in database
		db.CloseSubscription(userID)

		// Report results
		resultMsg := "🔧 Force Cancel Results:\n\n"
		if len(cancelled) > 0 {
			resultMsg += "✅ Cancelled subscriptions:\n"
			for _, id := range cancelled {
				resultMsg += fmt.Sprintf("• %s\n", id)
			}
		}
		if len(errors) > 0 {
			resultMsg += "\n❌ Errors:\n"
			for _, err := range errors {
				resultMsg += fmt.Sprintf("• %s\n", err)
			}
		}
		if len(cancelled) == 0 && len(errors) == 0 {
			resultMsg += "⚠️ No subscriptions found to cancel"
		}

		msg := tgbotapi.NewMessage(chatID, resultMsg)
		bot.Send(msg)
	case "/list_subs":
		// List all subscriptions for user
		subs, err := stripeService.ListAllSubscriptionsForUser(userID)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error listing subscriptions: %v", err))
			bot.Send(msg)
			return
		}

		if len(subs) == 0 {
			msg := tgbotapi.NewMessage(chatID, "📋 No subscriptions found in Stripe for your user ID")
			bot.Send(msg)
			return
		}

		resultMsg := fmt.Sprintf("📋 Found %d subscription(s):\n\n", len(subs))
		for i, sub := range subs {
			resultMsg += fmt.Sprintf("%d. ID: %s\n   Status: %s\n   Created: %s\n\n",
				i+1,
				sub.ID,
				sub.Status,
				time.Unix(sub.Created, 0).Format("2006-01-02 15:04:05"))
		}

		msg := tgbotapi.NewMessage(chatID, resultMsg)
		bot.Send(msg)
	case "Settings", "⚙️ Settings":
		msg := tgbotapi.NewMessage(chatID, "⚙️ Settings")
		msg.ReplyMarkup = getSettingsKeyboard(isPremiumUser(userID))
		bot.Send(msg)
	case "🎁 Enter Promo Code":
		msg := tgbotapi.NewMessage(chatID, "🎁 Enter Promo Code for free access:")
		bot.Send(msg)
		state.Stage = StageAwaitingFreePromo
	case "Cancel Subscription":
		// Legacy handler - execute cancellation
		executeCancelSubscription(bot, userID, chatID, logger)
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
				msg.ReplyMarkup = getMainMenuKeyboard(isPremiumUser(userID))
				bot.Send(msg)
				state.Stage = StageInitial
			} else {
				msg := tgbotapi.NewMessage(chatID, "Invalid timeframe. Please choose from the list:")
				bot.Send(msg)
				sendTimeframeMenu(bot, chatID)
			}
		case StageAwaitingFreePromo: // Expecting free promo code
			promoCode := strings.TrimSpace(message.Text)
			if promoCode == "" {
				msg := tgbotapi.NewMessage(chatID, "Enter promo code or use main menu:")
				msg.ReplyMarkup = getMainMenuKeyboard(isPremiumUser(userID))
				bot.Send(msg)
				state.Stage = StageInitial
				return
			}

			// Check hardcoded promo codes
			if promoCode == FREE_PROMO_CODE {
				// Check if user already used this promo code
				hasUsed, err := db.HasUsedPromoCode(userID, promoCode)
				if err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error checking promo code usage")
					msg := tgbotapi.NewMessage(chatID, "❌ Error checking promo code. Try again later.")
					bot.Send(msg)
					return
				}

				if hasUsed {
					msg := tgbotapi.NewMessage(chatID, "❌ You have already used this promo code!\n\nEach promo code can only be used once per user.")
					msg.ReplyMarkup = getMainMenuKeyboard(isPremiumUser(userID))
					bot.Send(msg)
					state.Stage = StageInitial
					return
				}

				// Grant free premium access
				if state.Symbol == "" || state.Interval == "" {
					state.Symbol = "EUR/USD"
					state.Interval = "5min"
				}

				// Create subscription in database with accepted status
				_, err = db.CreateSubscription(userID, chatID, state.Symbol, state.Interval)
				if err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating free subscription")
				} else {
					// Immediately activate subscription
					err = db.UpdateSubscriptionStatus(userID, models.PaymentStatusAccepted, "promo_"+promoCode)
					if err != nil {
						logger.Error().Err(err).Int64("user_id", userID).Msg("Error activating free subscription")
					}
				}

				state.Stage = StagePremium
				msg := tgbotapi.NewMessage(chatID, "🎉 Congratulations! Promo code accepted!\n\n✅ Premium subscription activated FREE!\n🔮 Now you can get predictions!")
				msg.ReplyMarkup = getMainMenuKeyboard(true)
				bot.Send(msg)
			} else if promoCode == FREE24_PROMO_CODE {
				// Check if user already used this promo code
				hasUsed, err := db.HasUsedPromoCode(userID, promoCode)
				if err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error checking promo code usage")
					msg := tgbotapi.NewMessage(chatID, "❌ Error checking promo code. Try again later.")
					bot.Send(msg)
					return
				}

				if hasUsed {
					msg := tgbotapi.NewMessage(chatID, "❌ You have already used this promo code!\n\nEach promo code can only be used once per user.")
					msg.ReplyMarkup = getMainMenuKeyboard(isPremiumUser(userID))
					bot.Send(msg)
					state.Stage = StageInitial
					return
				}

				// Grant 24-hour free access
				if state.Symbol == "" || state.Interval == "" {
					state.Symbol = "EUR/USD"
					state.Interval = "5min"
				}

				// Create subscription with 24-hour expiry
				_, err = db.CreateSubscriptionWithCustomExpiry(userID, chatID, state.Symbol, state.Interval, time.Now().Add(24*time.Hour))
				if err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating 24h subscription")
				} else {
					// Immediately activate subscription
					err = db.UpdateSubscriptionStatus(userID, models.PaymentStatusAccepted, "promo_"+promoCode)
					if err != nil {
						logger.Error().Err(err).Int64("user_id", userID).Msg("Error activating 24h subscription")
					}
				}

				state.Stage = StagePremium
				msg := tgbotapi.NewMessage(chatID, "🎉 Congratulations! FREE24 activated!\n\n✅ Premium access for 24 hours!\n🔮 Start getting predictions!\n\n⏰ Your access will expire in 24 hours.")
				msg.ReplyMarkup = getMainMenuKeyboard(true)
				bot.Send(msg)
			} else if promoCode == TEST_PROMO_CODE {
				// Check if user already used this promo code
				hasUsed, err := db.HasUsedPromoCode(userID, promoCode)
				if err != nil {
					logger.Error().Err(err).Int64("user_id", userID).Msg("Error checking promo code usage")
					msg := tgbotapi.NewMessage(chatID, "❌ Error checking promo code. Try again later.")
					bot.Send(msg)
					return
				}

				if hasUsed {
					msg := tgbotapi.NewMessage(chatID, "❌ You have already used this promo code!\n\nEach promo code can only be used once per user.")
					msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
					bot.Send(msg)
					state.Stage = StageInitial
					return
				}

				// Set default pair and interval if not set
				if state.Symbol == "" || state.Interval == "" {
					state.Symbol = "EUR/USD"
					state.Interval = "5min"
				}

				// Save promo code in state
				state.PromoCode = promoCode

				msg := tgbotapi.NewMessage(chatID, "🎉 TEST promo code accepted!\n\n💳 Now you'll be redirected to payment with special pricing.\n\n🔄 Creating payment session...")
				bot.Send(msg)

				// Proceed to payment with special price
				proceedToPaymentWithPromo(bot, userID, chatID, state, logger, promoCode)
			} else {
				msg := tgbotapi.NewMessage(chatID, "❌ Invalid promo code!\n\nTry again or use main menu:")
				msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
				bot.Send(msg)
				state.Stage = StageInitial
			}
		default:
			msg := tgbotapi.NewMessage(chatID, "Please use the menu buttons to interact with the bot.")
			msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
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

	// Handle pair and timeframe selections
	if strings.HasPrefix(data, "pair_") {
		pair := strings.TrimPrefix(data, "pair_")
		state.Symbol = pair
		state.Stage = StageAwaitingInterval

		// Acknowledge the callback
		bot.Request(tgbotapi.NewCallback(callback.ID, fmt.Sprintf("Selected %s", pair)))

		// Send timeframe menu
		sendTimeframeMenu(bot, chatID)
	} else if strings.HasPrefix(data, "interval_") {
		interval := strings.TrimPrefix(data, "interval_")
		state.Interval = interval

		// Acknowledge the callback
		bot.Request(tgbotapi.NewCallback(callback.ID, fmt.Sprintf("Selected %s", interval)))

		// Now both pair and timeframe are selected, ask user what to do next
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Selected: %s on %s timeframe\n\nWhat would you like to do?", state.Symbol, state.Interval))

		// Show run prediction button
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔮 Run Prediction", "run_prediction"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("← Back to Main Menu", "main_menu"),
			),
		)
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	} else if data == "run_prediction" {
		// Check subscription status before running prediction
		sub, err := db.GetSubscription(userID)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error retrieving subscription")
			bot.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "❌ Error checking subscription"))
			return
		}

		if sub == nil || sub.Status != models.PaymentStatusAccepted {
			// User needs to subscribe
			bot.Request(tgbotapi.NewCallback(callback.ID, "Subscription required"))
			msg := tgbotapi.NewMessage(chatID, "To run predictions you need a premium subscription. The subscription costs $4.99 per month.")
			msg.ReplyMarkup = getPaymentKeyboard()
			bot.Send(msg)
			state.Stage = StageAwaitingPayment
		} else {
			// User has active subscription, run prediction
			bot.Request(tgbotapi.NewCallback(callback.ID, "Running prediction..."))
			runPrediction(bot, chatID, state, logger)
			// Update last predicted time
			if err := db.UpdateLastPredicted(userID); err != nil {
				logger.Error().Err(err).Int64("user_id", userID).Msg("Error updating last predicted time")
			}
		}
	} else if data == "main_menu" {
		bot.Request(tgbotapi.NewCallback(callback.ID, ""))
		msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
		bot.Send(msg)
		state.Stage = StageInitial
	} else if data == "subscribe_now" {
		// Handle subscription
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before subscribing.")
			bot.Send(msg)
			sendCurrencyPairMenu(bot, chatID)
			state.Stage = StageAwaitingPair
			return
		}

		// Go directly to payment
		proceedToPayment(bot, userID, chatID, state, logger)
	} else if data == "cancel_subscription" {
		// Handle subscription cancellation from settings
		logger.Info().Int64("user_id", userID).Msg("User requested subscription cancellation from settings")

		// Get subscription from database
		sub, err := db.GetSubscription(userID)
		if err != nil {
			logger.Error().Err(err).Int64("user_id", userID).Msg("Error getting subscription from database")
			bot.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "❌ Error retrieving subscription from database"))
			return
		}

		if sub == nil {
			logger.Warn().Int64("user_id", userID).Msg("No subscription found in database")
			bot.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "❌ Subscription not found"))
			return
		}

		if sub.Status == models.PaymentStatusClosed {
			bot.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "ℹ️ Subscription already cancelled"))
			return
		}

		// Show confirmation dialog
		confirmMsg := tgbotapi.NewMessage(chatID, "⚠️ Are you sure you want to cancel the subscription?\n\nThis action cannot be undone.")
		confirmMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Yes, cancel", "confirm_cancel_subscription"),
				tgbotapi.NewInlineKeyboardButtonData("❌ No, keep it", "settings_menu"),
			),
		)
		bot.Send(confirmMsg)
	} else if data == "confirm_cancel_subscription" {
		// Execute subscription cancellation (same logic as "Cancel Subscription" in handleMessage)
		executeCancelSubscription(bot, userID, chatID, logger)
	} else if data == "settings_menu" {
		// Return to settings menu
		isPremium := shouldShowPremiumMenu(userID)
		msg := tgbotapi.NewMessage(chatID, "⚙️ Settings")
		msg.ReplyMarkup = getSettingsKeyboard(isPremium)
		bot.Send(msg)
	} else if data == "subscription_info" {
		// Show subscription information
		subscription, err := db.GetSubscription(userID)
		if err != nil {
			bot.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "❌ Error retrieving subscription information"))
			return
		}

		if subscription == nil {
			bot.Request(tgbotapi.NewCallbackWithAlert(callback.ID, "❌ Subscription not found"))
			return
		}

		var statusText string
		switch subscription.Status {
		case "pending":
			statusText = "⏳ Pending"
		case "accepted":
			statusText = "✅ Active"
		case "closed":
			statusText = "❌ Cancelled"
		default:
			statusText = subscription.Status
		}

		text := fmt.Sprintf(
			"📊 *Subscription Information*\n\n"+
				"💳 Stripe ID: `%s`\n"+
				"📅 Created: %s\n"+
				"📋 Status: %s\n"+
				"💰 Subscription: Premium",
			subscription.PaymentID,
			subscription.CreatedAt.Format("2006-01-02 15:04"),
			statusText,
		)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("← Back to Settings", "settings_menu"),
			),
		)

		editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, text)
		editMsg.ParseMode = "Markdown"
		editMsg.ReplyMarkup = &keyboard
		bot.Send(editMsg)
	} else if data == "about_info" {
		// Show about information
		text := "🤖 *About This Bot*\n\n" +
			"💡 *Premium subscription includes:*\n" +
			"• ⚡ Faster processing\n" +
			"• �� Advanced features\n" +
			"• 🔄 Priority support\n" +
			"• 📊 Detailed analytics\n\n" +
			"👥 Support: @support\n" +
			"📱 Version: 1.0.0"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("← Back to Settings", "settings_menu"),
			),
		)

		editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, text)
		editMsg.ParseMode = "Markdown"
		editMsg.ReplyMarkup = &keyboard
		bot.Send(editMsg)
	} else if data == "separator_crypto" {
		// Just acknowledge separator button without action
		bot.Request(tgbotapi.NewCallback(callback.ID, "🚀 Crypto section"))
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
			"\n🚨Attention: Refusal of responsibility\n\nThe trading signals provided are intended exclusively for information and educational purposes. They are not an investment recommendation and cannot be considered as a financial council.\n\nThe user agrees that: \n• All trade decisions are made by him at his own risk. \n• He undertakes to use no more than 1-2% of the deposit per transaction. \n• He realizes that trade in financial markets is associated with a high level of risk. \n• The company is not responsible for the possible losses incurred as a result of the use of signals.\n\nUsing this service, you confirm that you are familiar with the risks and take full responsibility for your actions.\n\nHow to Read a Signal: Educational Guide\n\nEach signal you receive includes important data. Here's how to interpret it:\n\n⸻\n\n1. Direction\nThis shows whether the model expects the price to go BUY (long) or SELL (short).\nExample: Direction: SELL means you may consider selling the asset.\n\n⸻\n\n2. Confidence\nThis indicates how strong the model's prediction is.\n • LOW: Less reliable, trade with caution.\n • MEDIUM: Moderate confidence.\n • HIGH: Strong signal with higher probability.\n\nYou should only trade when the confidence is HIGH or very close to it.\n\n⸻\n\n3. Score\nThis is a numerical value showing the strength and direction of the signal.\n • Positive scores (e.g., +5.0) = Buying pressure (BUY)\n • Negative scores (e.g., -5.0) = Selling pressure (SELL)\n • Values around 0 = Unclear direction, avoid trading.\n\nAs a rule of thumb:\n • Score > +4 → Strong Buy\n • Score < -4 → Strong Sell\n\n⸻\n\n4. Market Regime & Volatility\nThis tells you the current market conditions:\n • Trending: Prices are moving in one direction (up or down).\n • Ranging: Prices are moving sideways (no strong trend).\n • Volatility shows how active the market is. Higher volatility = more movement, more risk.\n\nUse this to decide whether it's a good time to enter a trade.\n\n⸻\n\n5. Indicators\nYou'll see technical indicators like RSI, MACD, Bollinger Bands, and more. These confirm the signal.\nFor example:\n • RSI under 50 = bearish pressure\n • MACD negative = bearish trend\n • Price near resistance = higher chance of reversal\n\n⸻\n\n6. Decision Factors\nThis section lists key technical signals the model uses to make its decision:\n • Patterns like Shooting Star, Double Top, Engulfing = Price reversal signals\n • Rejection at support/resistance zones\n • Indicator alignment (multiple indicators agreeing)\n\n⸻\n\n7. Trading Recommendations\nIncludes a sample trade setup:\n • Action: What to do (BUY/SELL)\n • Entry Price: Where to enter the trade\n • Stop Loss: Where to limit your loss\n • Take Profit: Where to close with a profit\n • Risk/Reward Ratio: Balance between risk and reward\n • Recommended Position Size: Based on 1% risk of your total capital\n\nYou're responsible for managing your risk.\nDo not trade emotionally or use high leverage. Stick to 1-2% risk per trade.\n\n⸻\n\nNote:\nThis is not financial advice. You trade at your own risk. For deeper knowledge, check out our PDF guide:\n\nhttps://t.me/Trade_Plus_Online_Bot")
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
				tgbotapi.NewKeyboardButton("🔮 Run Prediction"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⚙️ Settings"),
				tgbotapi.NewKeyboardButton("📊 Select Pair & Timeframe"),
			),
		)
	}

	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Select Pair & Timeframe"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("💎 Subscribe"),
			tgbotapi.NewKeyboardButton("🎁 Enter Promo Code"),
		),
	)
}

// getPaymentKeyboard returns the keyboard for payment options
func getPaymentKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Subscribe ($4.99/month)", "subscribe_now"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back to Main Menu", "main_menu"),
		),
	)
}

// getSettingsKeyboard returns the keyboard for settings menu
func getSettingsKeyboard(isPremium bool) tgbotapi.InlineKeyboardMarkup {
	var buttons [][]tgbotapi.InlineKeyboardButton

	if isPremium {
		// Add subscription management options for premium users
		buttons = append(buttons,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📋 Subscription Info", "subscription_info"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Subscription", "cancel_subscription"),
			),
		)
	}

	// Add common settings
	buttons = append(buttons,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ About", "about_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back to Main Menu", "main_menu"),
		),
	)

	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
}

// sendCurrencyPairMenu displays currency pair selection as inline buttons
func sendCurrencyPairMenu(bot *tgbotapi.BotAPI, chatID int64) {
	var keyboard [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	// Split pairs into forex and crypto for better organization
	forexPairs := []string{
		"EUR/USD", "GBP/USD", "USD/JPY", "AUD/USD",
		"USD/CAD", "USD/CHF", "NZD/USD", "EUR/GBP",
		"EUR/JPY", "GBP/JPY", "AUD/CAD", "EUR/CAD",
		"XBR/USD", "XAU/USD", "XAG/USD",
	}

	cryptoPairs := []string{
		"BTC/USD", "ETH/USD", "SOL/USD", "XRP/USD",
		"ADA/USD", "AAVE/USD", "BNB/USD", "DOT/USD",
	}

	// Add forex pairs (2 per row)
	for i, pair := range forexPairs {
		if i%2 == 0 && i > 0 {
			keyboard = append(keyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(pair, "pair_"+pair))
	}

	// Add the last forex row if it has any buttons
	if len(row) > 0 {
		keyboard = append(keyboard, row)
		row = []tgbotapi.InlineKeyboardButton{}
	}

	// Add separator for crypto section
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("━━━ 🚀 CRYPTO ━━━", "separator_crypto"),
	})

	// Add crypto pairs (2 per row)
	for i, pair := range cryptoPairs {
		if i%2 == 0 && i > 0 {
			keyboard = append(keyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(pair, "pair_"+pair))
	}

	// Add the last crypto row if it has any buttons
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
	logger.Debug().Int("candles_count", len(candles)).Msg("Starting market regime classification")
	regime, err := anomaly.EnhancedMarketRegimeClassification(candles)
	if err != nil {
		logger.Error().Err(err).Int("candles_count", len(candles)).Msg("Failed to classify market regime")
		regime = &models.MarketRegime{
			Type:             "UNKNOWN",
			Strength:         0,
			Direction:        "NEUTRAL",
			VolatilityLevel:  "NORMAL",
			MomentumStrength: 0,
			LiquidityRating:  "NORMAL",
			PriceStructure:   "UNKNOWN",
		}
	} else {
		logger.Debug().Str("regime_type", regime.Type).Float64("strength", regime.Strength).Str("volatility", regime.VolatilityLevel).Msg("Market regime calculated successfully")
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
	if prediction.Direction == "BUY" {
		directionEmoji = "🔼"
	} else if prediction.Direction == "SELL" {
		directionEmoji = "🔽"
	}

	resultText.WriteString(fmt.Sprintf("*Direction:* %s %s\n", directionEmoji, prediction.Direction))
	resultText.WriteString(fmt.Sprintf("*Confidence:* %s\n", prediction.Confidence))
	resultText.WriteString(fmt.Sprintf("*Score:* %.2f\n\n", prediction.Score))

	// Market regime
	resultText.WriteString(fmt.Sprintf("*Market Regime:* %s\n", regime.Type))
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

// isPremiumUser checks if user has an active subscription
func isPremiumUser(userID int64) bool {
	sub, err := db.GetSubscription(userID)
	if err != nil {
		return false
	}

	if sub == nil {
		return false
	}

	return sub.Status == models.PaymentStatusAccepted
}

// shouldShowPremiumMenu determines if premium menu should be shown based on user state and subscription
func shouldShowPremiumMenu(userID int64) bool {
	// First check if user actually has premium subscription
	if isPremiumUser(userID) {
		return true
	}

	// Check if user is in premium state (after successful promo code or payment)
	if state, exists := userStates[userID]; exists {
		// Show premium menu if user is in premium stage or has TEST promo active
		return state.Stage == StagePremium || (state.Stage == StageAwaitingPayment && state.PromoCode == "TEST")
	}

	return false
}

// executeCancelSubscription handles the subscription cancellation logic
func executeCancelSubscription(bot *tgbotapi.BotAPI, userID, chatID int64, logger *zerolog.Logger) {
	logger.Info().Int64("user_id", userID).Msg("User requested subscription cancellation")

	// Get subscription from database
	sub, err := db.GetSubscription(userID)
	if err != nil {
		logger.Error().Err(err).Int64("user_id", userID).Msg("Error getting subscription from database")
		msg := tgbotapi.NewMessage(chatID, "❌ Error retrieving subscription from database")
		bot.Send(msg)
		return
	}

	if sub == nil {
		logger.Warn().Int64("user_id", userID).Msg("No subscription found in database")
		msg := tgbotapi.NewMessage(chatID, "❌ Subscription not found")
		bot.Send(msg)
		return
	}

	if sub.Status == models.PaymentStatusClosed {
		msg := tgbotapi.NewMessage(chatID, "ℹ️ Subscription already cancelled")
		bot.Send(msg)
		return
	}

	// Send processing message
	processingMsg := tgbotapi.NewMessage(chatID, "🔄 Cancelling subscription...")
	sentMsg, _ := bot.Send(processingMsg)

	var stripeSuccess bool = false
	var stripeID string = ""

	// Try to cancel in Stripe
	logger.Info().Int64("user_id", userID).Str("stripe_subscription_id", sub.StripeSubscriptionID).Msg("Starting Stripe cancellation process")

	if sub.StripeSubscriptionID != "" {
		logger.Info().Str("subscription_id", sub.StripeSubscriptionID).Msg("Attempting to cancel with existing Stripe ID")
		// Try with existing ID
		if err := stripeService.CancelSubscription(sub.StripeSubscriptionID); err != nil {
			logger.Error().Err(err).Str("subscription_id", sub.StripeSubscriptionID).Msg("Failed to cancel existing subscription")
			// Check if it's a meaningful error
			if !strings.Contains(err.Error(), "No such subscription") && !strings.Contains(err.Error(), "already canceled") {
				logger.Error().Err(err).Str("subscription_id", sub.StripeSubscriptionID).Msg("Serious error cancelling existing subscription")
			} else {
				logger.Warn().Err(err).Str("subscription_id", sub.StripeSubscriptionID).Msg("Subscription not found or already cancelled")
			}
		} else {
			stripeSuccess = true
			stripeID = sub.StripeSubscriptionID
			logger.Info().Str("subscription_id", stripeID).Msg("Successfully cancelled subscription with existing ID")
		}
	}

	// If no existing ID or cancellation failed, try to find subscription
	if !stripeSuccess {
		logger.Info().Int64("user_id", userID).Msg("Searching for subscription in Stripe")

		// Try to find by user ID
		if stripeSubscription, err := stripeService.FindSubscriptionByUserID(userID); err == nil {
			logger.Info().Str("subscription_id", stripeSubscription.ID).Msg("Found subscription by user ID")
			if err := stripeService.CancelSubscription(stripeSubscription.ID); err == nil {
				stripeSuccess = true
				stripeID = stripeSubscription.ID
				// Update database with found ID
				db.UpdateStripeSubscriptionID(userID, stripeSubscription.ID)
				logger.Info().Str("subscription_id", stripeID).Msg("Successfully cancelled found subscription")
			} else {
				logger.Error().Err(err).Str("subscription_id", stripeSubscription.ID).Msg("Failed to cancel found subscription")
			}
		} else {
			logger.Warn().Err(err).Int64("user_id", userID).Msg("Could not find subscription by user ID")

			// Try advanced search
			searchAfter := sub.CreatedAt.Unix() - 3600
			logger.Info().Int64("search_after", searchAfter).Msg("Trying advanced search")

			if stripeSubscription, err := stripeService.FindSubscriptionAdvanced(userID, searchAfter); err == nil {
				logger.Info().Str("subscription_id", stripeSubscription.ID).Msg("Found subscription through advanced search")
				if err := stripeService.CancelSubscription(stripeSubscription.ID); err == nil {
					stripeSuccess = true
					stripeID = stripeSubscription.ID
					// Update database with found ID
					db.UpdateStripeSubscriptionID(userID, stripeSubscription.ID)
					logger.Info().Str("subscription_id", stripeID).Msg("Successfully cancelled subscription found through advanced search")
				} else {
					logger.Error().Err(err).Str("subscription_id", stripeSubscription.ID).Msg("Failed to cancel subscription found through advanced search")
				}
			} else {
				logger.Error().Err(err).Int64("user_id", userID).Msg("Advanced search also failed to find subscription")
			}
		}
	}

	// Cancel in database
	if err := db.CloseSubscription(userID); err != nil {
		logger.Error().Err(err).Int64("user_id", userID).Msg("Error cancelling subscription in database")
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, "❌ Error cancelling subscription in database")
		bot.Send(editMsg)
		return
	}

	// Update user state
	if state, exists := userStates[userID]; exists {
		state.Stage = StageInitial
		userStates[userID] = state
	}

	// Send result message
	var resultMsg string
	if stripeSuccess {
		resultMsg = fmt.Sprintf("✅ Subscription successfully cancelled!\n\n💳 Stripe ID: %s\n📱 Bot Status: cancelled\n\n🛡️ No recurring charges will occur.\n\nThank you for using our service!", stripeID)
	} else {
		resultMsg = "⚠️ Subscription cancelled in bot but not found in Stripe.\n\n🚨 IMPORTANT: Recurring charges may still occur!\n\n📞 URGENT: Contact support:\n• Message bot support\n• Or contact your bank to block recurring charges\n\n💳 Stripe ID: " + sub.StripeSubscriptionID
	}

	// Send result message and return to main menu
	resultMessage := tgbotapi.NewMessage(chatID, resultMsg)
	bot.Send(resultMessage)

	menuMsg := tgbotapi.NewMessage(chatID, "Main menu:")
	menuMsg.ReplyMarkup = getMainMenuKeyboard(false)
	bot.Send(menuMsg)

	logger.Info().Int64("user_id", userID).Bool("stripe_success", stripeSuccess).Str("stripe_id", stripeID).Msg("Subscription cancellation completed")
}

// proceedToPayment handles the payment process with optional promo code
func proceedToPayment(bot *tgbotapi.BotAPI, userID, chatID int64, state *UserState, logger *zerolog.Logger) {
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

	// Create regular checkout session
	sessionID, paymentURL, err := stripeService.CreateCheckoutSession(userID, state.Symbol, state.Interval)

	if err != nil {
		logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating Stripe session")

		var errorMsg string
		if strings.Contains(err.Error(), "STRIPE_SUBSCRIPTION_PRICE_ID not set") {
			errorMsg = "Payment system configuration error. Please contact support."
		} else if strings.Contains(err.Error(), "TELEGRAM_BOT_USERNAME not set") {
			errorMsg = "Bot configuration error. Please contact support."
		} else if strings.Contains(err.Error(), "No such price") {
			errorMsg = "Invalid subscription price configuration. Please contact support."
		} else if strings.Contains(err.Error(), "Invalid API key") {
			errorMsg = "Payment system authentication error. Please contact support."
		} else {
			errorMsg = fmt.Sprintf("Payment system error: %v\n\nPlease try again or contact support.", err)
		}

		msg := tgbotapi.NewMessage(chatID, errorMsg)
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
		"Please complete your payment to access premium predictions.",
	)

	// Add payment URL button
	editMsg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonURL("💳 Pay Now", paymentURL),
			},
		},
	}

	bot.Send(editMsg)

	// Add follow-up message
	followUp := tgbotapi.NewMessage(chatID, "After completing payment, return to this chat. Your subscription will be activated automatically.")
	bot.Send(followUp)

	logger.Info().Int64("user_id", userID).Str("session_id", sessionID).Str("payment_url", paymentURL).Msg("Created payment session")
}

// proceedToPaymentWithPromo handles the payment process with promo code
func proceedToPaymentWithPromo(bot *tgbotapi.BotAPI, userID, chatID int64, state *UserState, logger *zerolog.Logger, promoCode string) {
	// Send a loading message
	loadingMsg := tgbotapi.NewMessage(chatID, "Creating payment session with promo code...")
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

	// Create checkout session with promo code
	sessionID, paymentURL, err := stripeService.CreateCheckoutSessionWithPromo(userID, state.Symbol, state.Interval, promoCode)

	if err != nil {
		logger.Error().Err(err).Int64("user_id", userID).Msg("Error creating Stripe session with promo")
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Payment system error: %v\n\nPlease try again or contact support.", err))
		bot.Send(msg)
		return
	}

	// Save payment info in user state
	state.PaymentURL = paymentURL
	state.SessionID = sessionID
	state.Stage = StageAwaitingPayment

	// Edit the loading message to provide payment instructions
	var editMsg tgbotapi.EditMessageTextConfig
	if sentMsg.MessageID != 0 {
		editMsg = tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID,
			fmt.Sprintf("🎯 %s promo activated!\n\n💳 Please complete your payment:\n%s\n\n✅ Click the link above to pay\n💰 Special pricing applied!\n⏰ Link expires in 30 minutes\n\n🔙 To cancel, use /cancel", promoCode, paymentURL))
	} else {
		// Fallback if we couldn't get message ID
		msg := tgbotapi.NewMessage(chatID,
			fmt.Sprintf("🎯 %s promo activated!\n\n💳 Please complete your payment:\n%s\n\n✅ Click the link above to pay\n💰 Special pricing applied!\n⏰ Link expires in 30 minutes\n\n🔙 To cancel, use /cancel", promoCode, paymentURL))
		bot.Send(msg)
		return
	}

	bot.Send(editMsg)

	// Send new menu with premium buttons
	menuMsg := tgbotapi.NewMessage(chatID, "You now have access to premium features! Use the menu below:")
	menuMsg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
	bot.Send(menuMsg)

	logger.Info().Int64("user_id", userID).Str("session_id", sessionID).Str("promo_code", promoCode).Msg("Created payment session with promo code")
}

// handlePayment handles payment-related actions
func handlePayment(bot *tgbotapi.BotAPI, userID, chatID int64, state *UserState, logger *zerolog.Logger) {
	msg := tgbotapi.NewMessage(chatID, "Waiting for payment completion. If you have already paid, please wait a few minutes for subscription activation.")
	msg.ReplyMarkup = getMainMenuKeyboard(shouldShowPremiumMenu(userID))
	bot.Send(msg)
}

// schedulePromoNotification планирует отправку уведомления ровно через 12 часов после истечения
func schedulePromoNotification(bot *tgbotapi.BotAPI, promoCode string, expirationDate time.Time, logger *zerolog.Logger) {
	// Время отправки = дата истечения + 12 часов
	notificationTime := expirationDate.Add(12 * time.Hour)

	// Вычисляем, через сколько времени отправлять
	timeUntilNotification := time.Until(notificationTime)

	// Если время уже прошло, отправляем сразу
	if timeUntilNotification <= 0 {
		logger.Info().
			Str("promo_code", promoCode).
			Time("expiration_date", expirationDate).
			Time("should_have_notified_at", notificationTime).
			Msg("Promo notification time already passed - sending immediately")

		// Отправляем сразу
		sendPromoNotification(bot, promoCode, expirationDate, logger)
		return
	}

	logger.Info().
		Str("promo_code", promoCode).
		Time("will_notify_at", notificationTime).
		Dur("in", timeUntilNotification).
		Msg("Scheduled promo expiration notification")

	// Ждем нужное время
	timer := time.NewTimer(timeUntilNotification)
	defer timer.Stop()

	<-timer.C

	// Отправляем уведомление
	sendPromoNotification(bot, promoCode, expirationDate, logger)
}

// sendPromoNotification отправляет уведомление об истечении промокода
func sendPromoNotification(bot *tgbotapi.BotAPI, promoCode string, expiredAt time.Time, logger *zerolog.Logger) {
	adminChatIDStr := os.Getenv("ADMIN_CHAT_ID")
	if adminChatIDStr == "" {
		logger.Info().
			Str("promo_code", promoCode).
			Time("expired_at", expiredAt).
			Msg("Promo code expired notification - no admin chat configured")
		return
	}

	adminChatID, err := strconv.ParseInt(adminChatIDStr, 10, 64)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid admin chat ID")
		return
	}

	// Вычисляем, сколько времени прошло с момента истечения
	timeSinceExpiration := time.Since(expiredAt)
	hoursExpired := int(timeSinceExpiration.Hours())

	var statusText string
	if hoursExpired >= 12 {
		statusText = fmt.Sprintf("Expired %d hours ago", hoursExpired)
	} else {
		statusText = "Just expired 12 hours ago"
	}

	messageText := fmt.Sprintf(
		"🚨 **Promo Code Expired**\n\n"+
			"**Code:** `%s`\n"+
			"**Expired:** %s\n"+
			"**Status:** %s\n\n"+
			"Users can no longer use this promotional code.",
		promoCode,
		expiredAt.Format("January 2, 2006 at 15:04 MST"),
		statusText,
	)

	msg := tgbotapi.NewMessage(adminChatID, messageText)
	msg.ParseMode = "Markdown"

	_, err = bot.Send(msg)
	if err != nil {
		logger.Error().Err(err).Str("promo_code", promoCode).Msg("Failed to send promo expiration notification")
	} else {
		logger.Info().
			Str("promo_code", promoCode).
			Int("hours_expired", hoursExpired).
			Msg("Promo expiration notification sent successfully")
	}
}
