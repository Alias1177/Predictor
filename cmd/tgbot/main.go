package main

import (
	"chi/Predictor/config"
	"chi/Predictor/internal/analyze"
	"chi/Predictor/internal/anomaly"
	"chi/Predictor/internal/calculate"
	"chi/Predictor/internal/gpt"
	"chi/Predictor/models"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

// Supported currency pairs and intervals
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

// UserState represents the current state of a user's interaction
type UserState struct {
	Stage        int       // 0: initial, 1: awaiting pair, 2: awaiting interval
	Symbol       string    // selected currency pair
	Interval     string    // selected time interval
	LastActivity time.Time // time of last activity
}

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, relying on actual environment variables")
	}
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

	// Start handling updates
	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, update.Message, &logger)
		} else if update.CallbackQuery != nil {
			handleCallback(bot, update.CallbackQuery, &logger)
		}
	}
}

// handleMessage processes incoming text messages
func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, logger *zerolog.Logger) {
	userID := message.From.ID
	chatID := message.Chat.ID

	// Get or initialize user state
	state, exists := userStates[userID]
	if !exists || message.Text == "/start" {
		userStates[userID] = &UserState{
			Stage:        0,
			LastActivity: time.Now(),
		}
		state = userStates[userID]

		// Send welcome message with main menu
		msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard()
		bot.Send(msg)
		return
	}

	// Update last activity
	state.LastActivity = time.Now()

	switch message.Text {
	case "/start", "Main Menu":
		msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard()
		bot.Send(msg)
		state.Stage = 0
	case "Select Currency Pair":
		sendCurrencyPairMenu(bot, chatID)
		state.Stage = 1
	case "Select Timeframe":
		if state.Symbol == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select a currency pair first.")
			bot.Send(msg)
			sendCurrencyPairMenu(bot, chatID)
			state.Stage = 1
		} else {
			sendTimeframeMenu(bot, chatID)
			state.Stage = 2
		}
	case "Run Prediction":
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before running prediction.")
			bot.Send(msg)
			msg = tgbotapi.NewMessage(chatID, "What would you like to do?")
			msg.ReplyMarkup = getMainMenuKeyboard()
			bot.Send(msg)
			state.Stage = 0
		} else {
			runPrediction(bot, chatID, state, logger)
		}
	default:
		// Handle other inputs based on current stage
		switch state.Stage {
		case 1: // Expecting currency pair
			if contains(supportedPairs, message.Text) {
				state.Symbol = message.Text
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected pair: %s\nNow select a timeframe.", message.Text))
				bot.Send(msg)
				sendTimeframeMenu(bot, chatID)
				state.Stage = 2
			} else {
				msg := tgbotapi.NewMessage(chatID, "Invalid currency pair. Please choose from the list:")
				bot.Send(msg)
				sendCurrencyPairMenu(bot, chatID)
			}
		case 2: // Expecting interval
			if contains(supportedIntervals, message.Text) {
				state.Interval = message.Text
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected timeframe: %s\nYou can now run the prediction.", message.Text))
				// Attach the main menu keyboard without sending welcome text
				msg.ReplyMarkup = getMainMenuKeyboard()
				bot.Send(msg)
				state.Stage = 0
			} else {
				msg := tgbotapi.NewMessage(chatID, "Invalid timeframe. Please choose from the list:")
				bot.Send(msg)
				sendTimeframeMenu(bot, chatID)
			}
		default:
			msg := tgbotapi.NewMessage(chatID, "Please use the menu buttons to interact with the bot.")
			msg.ReplyMarkup = getMainMenuKeyboard()
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
			Stage:        0,
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
		state.Stage = 2
	} else if strings.HasPrefix(data, "interval_") {
		// Extract interval from callback data
		interval := strings.TrimPrefix(data, "interval_")
		state.Interval = interval

		// Send confirmation message with main menu keyboard but no welcome text
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Selected timeframe: %s\nYou can now run the prediction.", interval))
		msg.ReplyMarkup = getMainMenuKeyboard()
		bot.Send(msg)

		state.Stage = 0
	} else if data == "run_prediction" {
		if state.Symbol == "" || state.Interval == "" {
			msg := tgbotapi.NewMessage(chatID, "Please select both currency pair and timeframe before running prediction.")
			bot.Send(msg)
			msg = tgbotapi.NewMessage(chatID, "What would you like to do?")
			msg.ReplyMarkup = getMainMenuKeyboard()
			bot.Send(msg)
			state.Stage = 0
		} else {
			runPrediction(bot, chatID, state, logger)
		}
	} else if data == "main_menu" {
		msg := tgbotapi.NewMessage(chatID, "What would you like to do?")
		msg.ReplyMarkup = getMainMenuKeyboard()
		bot.Send(msg)
		state.Stage = 0
	}
}

// getMainMenuKeyboard returns the main menu keyboard
func getMainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
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

// sendMainMenu displays the main menu buttons with welcome message
func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Welcome to the Forex Predictor Bot! What would you like to do?")
	msg.ReplyMarkup = getMainMenuKeyboard()
	bot.Send(msg)
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
	regime := anomaly.EnhancedMarketRegimeClassification(candles)
	anomalyData := anomaly.DetectMarketAnomalies(candles)

	// Generate prediction
	direction, confidence, score, factors := analyze.EnhancedPrediction(
		candles, indicators, mtfData, regime, anomalyData, cfg)

	// Edit message to show loading status
	editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, "⏳ Analyzing market data...")
	bot.Send(editMsg)

	// Format the prediction message
	var resultText strings.Builder
	resultText.WriteString(fmt.Sprintf("*Prediction for %s (%s)*\n\n", state.Symbol, state.Interval))

	// Direction emoji
	directionEmoji := "⚖️"
	if direction == "UP" {
		directionEmoji = "🔼"
	} else if direction == "DOWN" {
		directionEmoji = "🔽"
	}

	resultText.WriteString(fmt.Sprintf("*Direction:* %s %s\n", directionEmoji, direction))
	resultText.WriteString(fmt.Sprintf("*Confidence:* %s\n", confidence))
	resultText.WriteString(fmt.Sprintf("*Score:* %.2f\n\n", score))

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
	for i, factor := range factors {
		resultText.WriteString(fmt.Sprintf("%d. %s\n", i+1, factor))
	}

	// Price data
	if len(candles) > 0 {
		currentPrice := candles[len(candles)-1].Close
		resultText.WriteString(fmt.Sprintf("\n*Current Price:* %.5f\n", currentPrice))
	}

	// Send the final result
	resultMsg := tgbotapi.NewMessage(chatID, resultText.String())
	resultMsg.ParseMode = "Markdown"
	bot.Send(resultMsg)

	// Try to get prediction from OpenAI if API key is available
	if cfg.OpenAIAPIKey != "" {
		aiPrompt := gpt.FormatPrompt(candles, cfg.Symbol)
		aiMsg := tgbotapi.NewMessage(chatID, "Getting AI analysis...")
		sentAiMsg, _ := bot.Send(aiMsg)

		// This is asynchronous so user doesn't have to wait
		go func() {
			// Create a buffer to capture GPT's output
			aiOutput := captureGPTOutput(cfg.OpenAIAPIKey, aiPrompt)

			// Update the message with AI prediction
			editAiMsg := tgbotapi.NewEditMessageText(
				chatID,
				sentAiMsg.MessageID,
				fmt.Sprintf("*AI Analysis:*\n\n%s", aiOutput),
			)
			editAiMsg.ParseMode = "Markdown"
			bot.Send(editAiMsg)
		}()
	}
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
func captureGPTOutput(apiKey, prompt string) string {
	// This is a simple wrapper around the gpt package that captures the output
	client := &gptClient{}
	return client.askGPT(apiKey, prompt)
}

// gptClient is a simple wrapper for gpt.AskGPT that captures the output
type gptClient struct{}

func (g *gptClient) askGPT(apiKey, prompt string) string {
	// Adapted from your gpt.AskGPT function to return the result as a string
	// instead of printing it to the console
	ctx := context.Background()
	result, err := gpt.ProcessGPT(ctx, apiKey, prompt)
	if err != nil {
		return "Error getting AI prediction: " + err.Error()
	}
	return result
}

// contains checks if a string exists in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
