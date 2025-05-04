package config

import (
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	TwelveAPIKey      string  `env:"TWELVE_API_KEY" envDefault:"-"`
	OpenAIAPIKey      string  `env:"OPENAI_API_KEY" envDefault:"-"`
	Symbol            string  `env:"SYMBOL" envDefault:"EUR/USD"`
	Interval          string  `env:"INTERVAL" envDefault:"5min"` // Changed from 3min to 5min (supported by API)
	CandleCount       int     `env:"CANDLE_COUNT" envDefault:"40"`
	RSIPeriod         int     `env:"RSI_PERIOD" envDefault:"9"`
	MACDFastPeriod    int     `env:"MACD_FAST_PERIOD" envDefault:"7"`
	MACDSlowPeriod    int     `env:"MACD_SLOW_PERIOD" envDefault:"14"`
	MACDSignalPeriod  int     `env:"MACD_SIGNAL_PERIOD" envDefault:"5"`
	BBPeriod          int     `env:"BB_PERIOD" envDefault:"16"`
	BBStdDev          float64 `env:"BB_STD_DEV" envDefault:"2.2"`
	EMAPeriod         int     `env:"EMA_PERIOD" envDefault:"10"`
	ADXPeriod         int     `env:"ADX_PERIOD" envDefault:"14"`
	ATRPeriod         int     `env:"ATR_PERIOD" envDefault:"14"`
	LogLevel          string  `env:"LOG_LEVEL" envDefault:"info"`
	RequestTimeout    int     `env:"REQUEST_TIMEOUT" envDefault:"30"` // seconds
	AdaptiveIndicator bool    `env:"ADAPTIVE_INDICATOR" envDefault:"true"`
	EnableBacktest    bool    `env:"ENABLE_BACKTEST" envDefault:"true"`
	BacktestDays      int     `env:"BACKTEST_DAYS" envDefault:"5"`
}

// Load initializes configuration from environment variables
func Load() (*Config, error) {
	// Load environment variables from .env file if present
	if err := godotenv.Load(); err != nil {
		log.Warn().Msg(".env file not found, relying on actual environment variables")
	}

	var cfg Config

	// Load values from environment variables
	cfg.TwelveAPIKey = os.Getenv("TWELVE_API_KEY")
	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	cfg.Symbol = getEnvWithDefault("SYMBOL", "EUR/USD")
	cfg.Interval = getEnvWithDefault("INTERVAL", "5min")
	cfg.CandleCount = getEnvIntWithDefault("CANDLE_COUNT", 40)
	cfg.RSIPeriod = getEnvIntWithDefault("RSI_PERIOD", 9)
	cfg.MACDFastPeriod = getEnvIntWithDefault("MACD_FAST_PERIOD", 7)
	cfg.MACDSlowPeriod = getEnvIntWithDefault("MACD_SLOW_PERIOD", 14)
	cfg.MACDSignalPeriod = getEnvIntWithDefault("MACD_SIGNAL_PERIOD", 5)
	cfg.BBPeriod = getEnvIntWithDefault("BB_PERIOD", 16)
	cfg.BBStdDev = getEnvFloatWithDefault("BB_STD_DEV", 2.2)
	cfg.EMAPeriod = getEnvIntWithDefault("EMA_PERIOD", 10)
	cfg.ADXPeriod = getEnvIntWithDefault("ADX_PERIOD", 14)
	cfg.ATRPeriod = getEnvIntWithDefault("ATR_PERIOD", 14)
	cfg.LogLevel = getEnvWithDefault("LOG_LEVEL", "info")
	cfg.RequestTimeout = getEnvIntWithDefault("REQUEST_TIMEOUT", 30)
	cfg.AdaptiveIndicator = getEnvBoolWithDefault("ADAPTIVE_INDICATOR", true)
	cfg.EnableBacktest = getEnvBoolWithDefault("ENABLE_BACKTEST", true)
	cfg.BacktestDays = getEnvIntWithDefault("BACKTEST_DAYS", 5)

	return &cfg, nil
}

// Helper functions for environment variable handling
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloatWithDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
