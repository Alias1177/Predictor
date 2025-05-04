package main

import (
	"chi/Predictor/internal/analysis/market"
	"chi/Predictor/internal/analysis/pattern"
	"chi/Predictor/internal/analysis/prediction"
	"chi/Predictor/internal/analysis/technical"
	"chi/Predictor/internal/api/openai"
	"chi/Predictor/internal/api/twelvedata"
	"chi/Predictor/internal/config"
	"chi/Predictor/internal/model"
	"chi/Predictor/internal/trading/backtest"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	setupSignalHandling(cancel)

	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// 2. Configure logging
	setupLogging(cfg.LogLevel)
	log.Info().Msg("Starting Forex Analyzer")

	// 3. Print configuration
	printConfig(cfg)

	// 4. Setup API clients
	twClientOpts := twelvedata.ClientOptions{
		APIKey:         cfg.TwelveAPIKey,
		RequestTimeout: time.Duration(cfg.RequestTimeout) * time.Second,
		RequestsPerSec: 5,
	}
	twClient := twelvedata.NewClient(twClientOpts)

	// 5. Run backtesting if enabled
	if cfg.EnableBacktest {
		runBacktesting(ctx, twClient, cfg)
	}

	// 6. Run live analysis
	runLiveAnalysis(ctx, twClient, cfg)
}

// setupSignalHandling configures signal handling for graceful shutdown
func setupSignalHandling(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msg("Shutdown signal received, exiting...")
		cancel()
		os.Exit(0)
	}()
}

// setupLogging configures the logger
func setupLogging(logLevel string) {
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	// Set log level from config
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	log.Logger = log.Logger.Level(level)
}

// printConfig outputs the current configuration
func printConfig(cfg *config.Config) {
	log.Info().
		Str("Symbol", cfg.Symbol).
		Str("Interval", cfg.Interval).
		Int("CandleCount", cfg.CandleCount).
		Int("RSIPeriod", cfg.RSIPeriod).
		Int("MACDFastPeriod", cfg.MACDFastPeriod).
		Int("MACDSlowPeriod", cfg.MACDSlowPeriod).
		Int("MACDSignalPeriod", cfg.MACDSignalPeriod).
		Int("BBPeriod", cfg.BBPeriod).
		Float64("BBStdDev", cfg.BBStdDev).
		Int("EMAPeriod", cfg.EMAPeriod).
		Int("ADXPeriod", cfg.ADXPeriod).
		Int("ATRPeriod", cfg.ATRPeriod).
		Bool("AdaptiveIndicator", cfg.AdaptiveIndicator).
		Bool("EnableBacktest", cfg.EnableBacktest).
		Int("BacktestDays", cfg.BacktestDays).
		Msg("Configuration loaded")
}

// runBacktesting performs backtesting analysis
func runBacktesting(ctx context.Context, client *twelvedata.Client, cfg *config.Config) {
	log.Info().Msg("Running backtesting...")
	engine := backtest.NewEngine(client, cfg)

	results, err := engine.Run(ctx, cfg.BacktestDays)
	if err != nil {
		log.Error().Err(err).Msg("Backtest failed")
		return
	}

	if results != nil {
		// Display results
		fmt.Println(engine.FormatResults(results))
	} else {
		log.Error().Msg("Backtest returned nil results")
	}
}

// runLiveAnalysis performs live market analysis
func runLiveAnalysis(ctx context.Context, client *twelvedata.Client, cfg *config.Config) {
	log.Info().Msg("Running live market analysis...")

	// 1. Fetch current market data
	log.Info().Msg("Fetching latest market data...")
	candles, err := client.GetCandles(ctx, cfg.Symbol, cfg.Interval, cfg.CandleCount)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch candles")
		return
	}

	// 2. Calculate technical indicators
	indicators := technical.CalculateAllIndicators(candles, cfg)
	if indicators == nil {
		log.Error().Msg("Failed to calculate indicators")
		return
	}

	// 3. Fetch multi-timeframe data
	mtfData, err := getMultiTimeframeData(ctx, client, cfg.Symbol)
	if err != nil {
		log.Warn().Err(err).Msg("Multi-timeframe data fetch failed")
	}

	// 4. Analyze market conditions
	regime := market.ClassifyMarketRegime(candles)
	anomaly := market.DetectMarketAnomalies(candles)
	patterns := pattern.IdentifyPriceActionPatterns(candles)

	// 5. Print market analysis
	printMarketAnalysis(candles, indicators, regime, anomaly, patterns)

	// 6. Generate prediction
	direction, confidence, score, factors := prediction.GeneratePrediction(
		candles, indicators, mtfData, regime, anomaly, cfg)

	// 7. Print prediction
	printPrediction(direction, confidence, score, factors)

	// 8. OpenAI analysis (if enabled)
	if cfg.OpenAIAPIKey != "" {
		runOpenAIAnalysis(ctx, candles, cfg)
	}
}

// getMultiTimeframeData fetches data for multiple timeframes
func getMultiTimeframeData(ctx context.Context, client *twelvedata.Client, symbol string) (map[string][]model.Candle, error) {
	// Define the timeframes we want to fetch
	timeframes := map[string]string{
		"1min":  "1min",
		"5min":  "5min",
		"15min": "15min",
	}

	result := make(map[string][]model.Candle)

	// Fetch each timeframe
	for name, interval := range timeframes {
		candles, err := client.GetCandles(ctx, symbol, interval, 30)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s candles: %w", name, err)
		}
		result[name] = candles
	}

	return result, nil
}

// printMarketAnalysis outputs the current market analysis
func printMarketAnalysis(candles []model.Candle, indicators *model.TechnicalIndicators,
	regime *model.MarketRegime, anomaly *model.AnomalyDetection, patterns []string) {

	fmt.Println("\n===== MARKET ANALYSIS =====")

	// Current price
	latest := candles[len(candles)-1]
	fmt.Printf("Current Price: %.5f (O: %.5f, H: %.5f, L: %.5f, C: %.5f)\n",
		latest.Close, latest.Open, latest.High, latest.Low, latest.Close)

	// Key indicators
	fmt.Printf("\nKey Indicators:\n")
	fmt.Printf("RSI: %.2f | ", indicators.RSI)
	fmt.Printf("MACD: %.5f, Signal: %.5f, Hist: %.5f | ",
		indicators.MACD, indicators.MACDSignal, indicators.MACDHist)
	fmt.Printf("ADX: %.2f (+DI: %.2f, -DI: %.2f)\n",
		indicators.ADX, indicators.PlusDI, indicators.MinusDI)

	// Bollinger Bands
	fmt.Printf("Bollinger Bands: Upper: %.5f, Middle: %.5f, Lower: %.5f\n",
		indicators.BBUpper, indicators.BBMiddle, indicators.BBLower)

	// Market regime
	fmt.Printf("\nMarket Regime: %s (Strength: %.2f)\n", regime.Type, regime.Strength)
	fmt.Printf("Direction: %s | Volatility: %s | Momentum: %.2f\n",
		regime.Direction, regime.VolatilityLevel, regime.MomentumStrength)
	fmt.Printf("Price Structure: %s\n", regime.PriceStructure)

	// Patterns
	if len(patterns) > 0 {
		fmt.Printf("\nDetected Patterns: %v\n", patterns)
	}

	// Anomalies
	if anomaly.IsAnomaly {
		fmt.Printf("\nANOMALY DETECTED: %s (Score: %.2f)\n",
			anomaly.AnomalyType, anomaly.AnomalyScore)
		fmt.Printf("Details: %s\n", anomaly.Details)
		fmt.Printf("Recommended Actions: %v\n", anomaly.RecommendedFlags)
	}

	// Support/Resistance
	if len(indicators.Support) > 0 {
		fmt.Printf("\nNearest Support Levels: ")
		for i, level := range indicators.Support {
			fmt.Printf("%.5f", level)
			if i < len(indicators.Support)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Println()
	}

	if len(indicators.Resistance) > 0 {
		fmt.Printf("Nearest Resistance Levels: ")
		for i, level := range indicators.Resistance {
			fmt.Printf("%.5f", level)
			if i < len(indicators.Resistance)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Println()
	}

	// Trade signal
	fmt.Printf("\nTrade Signal: %s\n", indicators.TradeSignal)
}

// printPrediction outputs the price prediction
func printPrediction(direction, confidence string, score float64, factors []string) {
	fmt.Println("\n===== PREDICTION =====")
	fmt.Printf("Direction: %s | Confidence: %s | Score: %.2f\n",
		direction, confidence, score)

	fmt.Println("\nFactors:")
	for _, factor := range factors {
		fmt.Printf("- %s\n", factor)
	}
	fmt.Println()
}

// runOpenAIAnalysis sends data to OpenAI for additional analysis
func runOpenAIAnalysis(ctx context.Context, candles []model.Candle, cfg *config.Config) {
	openAIClient := openai.NewClient(cfg.OpenAIAPIKey)
	prompt := openai.FormatCandlePrompt(cfg.Symbol, candles)

	response, err := openAIClient.GenerateCompletion(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("OpenAI API error")
		return
	}

	fmt.Println("\n===== AI ANALYSIS =====")
	fmt.Println(response)
}
