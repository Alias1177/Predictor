package models

import (
	"time"
)

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

// Candle represents a single price candle
type Candle struct {
	Datetime string  `json:"datetime"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   int64   `json:"volume,omitempty"`
}

// TwelveResponse represents the API response from Twelve Data
type TwelveResponse struct {
	Meta struct {
		Symbol   string `json:"symbol"`
		Interval string `json:"interval"`
	} `json:"meta"`
	Values []struct {
		Datetime string  `json:"datetime"`
		Open     float64 `json:"open,string"`
		High     float64 `json:"high,string"`
		Low      float64 `json:"low,string"`
		Close    float64 `json:"close,string"`
		Volume   int64   `json:"volume,string,omitempty"`
	} `json:"values"`
	Status string `json:"status"`
}

// TechnicalIndicators holds all calculated technical indicators
type TechnicalIndicators struct {
	RSI              float64   `json:"rsi"`
	MACD             float64   `json:"macd"`
	MACDSignal       float64   `json:"macd_signal"`
	MACDHist         float64   `json:"macd_hist"`
	BBUpper          float64   `json:"bb_upper"`
	BBMiddle         float64   `json:"bb_middle"`
	BBLower          float64   `json:"bb_lower"`
	EMA              float64   `json:"ema"`
	ATR              float64   `json:"atr"`
	ADX              float64   `json:"adx"`
	PlusDI           float64   `json:"plus_di"`
	MinusDI          float64   `json:"minus_di"`
	PriceChange      float64   `json:"price_change_pct"` // % change in last 5 candles
	VolumeChange     float64   `json:"volume_change_pct,omitempty"`
	Momentum         float64   `json:"momentum"`   // Current close - close n periods ago
	Trends           []string  `json:"trends"`     // Array of identified trends
	Support          []float64 `json:"support"`    // Potential support levels
	Resistance       []float64 `json:"resistance"` // Potential resistance levels
	Stochastic       float64   `json:"stochastic"` // Stochastic oscillator
	StochasticSignal float64   `json:"stochastic_signal"`
	OBV              float64   `json:"obv"` // On-Balance Volume
	VolatilityRatio  float64   `json:"volatility_ratio"`
	TradeSignal      string    `json:"trade_signal"` // STRONG_BUY, BUY, NEUTRAL, SELL, STRONG_SELL
}

// MarketRegime represents the current market conditions
type MarketRegime struct {
	Type             string  `json:"type"`     // TRENDING, RANGING, VOLATILE, CHOPPY
	Strength         float64 `json:"strength"` // 0-1 score indicating regime strength
	Direction        string  `json:"direction"`
	VolatilityLevel  string  `json:"volatility_level"`  // LOW, NORMAL, HIGH
	MomentumStrength float64 `json:"momentum_strength"` // 0-1 score
	LiquidityRating  string  `json:"liquidity_rating"`  // LOW, NORMAL, HIGH
	PriceStructure   string  `json:"price_structure"`   // TRENDING_UP, TRENDING_DOWN, RANGE_BOUND, BREAKOUT, BREAKDOWN
}

// AnomalyDetection contains information about market anomalies
type AnomalyDetection struct {
	IsAnomaly        bool     `json:"is_anomaly"`
	AnomalyType      string   `json:"anomaly_type,omitempty"` // PRICE_SPIKE, VOLUME_SPIKE, GAP, PATTERN_BREAK
	AnomalyScore     float64  `json:"anomaly_score"`          // 0-1 score
	Details          string   `json:"details,omitempty"`
	RecommendedFlags []string `json:"recommended_flags,omitempty"`
}

// PredictionResult stores the outcome of a prediction
type PredictionResult struct {
	Direction        string    `json:"direction"`
	Confidence       string    `json:"confidence"`
	Score            float64   `json:"score"`
	Factors          []string  `json:"factors"`
	Timestamp        time.Time `json:"timestamp"`
	PredictionID     string    `json:"prediction_id"`
	PredictionTarget time.Time `json:"prediction_target"` // When this prediction should be validated
	ActualOutcome    string    `json:"actual_outcome,omitempty"`
	WasCorrect       bool      `json:"was_correct,omitempty"`
}

// BacktestResults stores backtesting results
type BacktestResults struct {
	TotalTrades    int     `json:"total_trades"`
	WinningTrades  int     `json:"winning_trades"`
	LosingTrades   int     `json:"losing_trades"`
	WinPercentage  float64 `json:"win_percentage"`
	AverageGain    float64 `json:"average_gain"`
	AverageLoss    float64 `json:"average_loss"`
	MaxConsecutive struct {
		Wins  int `json:"wins"`
		Loses int `json:"loses"`
	} `json:"max_consecutive"`
	MarketRegimePerformance map[string]float64 `json:"market_regime_performance"`
	TimeframePerformance    map[string]float64 `json:"timeframe_performance"`
	DetailedResults         []PredictionResult `json:"detailed_results"`
	ProfitFactor            float64            `json:"profit_factor"`
	MaxDrawdown             float64            `json:"max_drawdown"`
	SharpeRatio             float64            `json:"sharpe_ratio"`
	EquityCurve             []float64          `json:"equity_curve,omitempty"`
	EquityGrowthPercent     float64            `json:"equity_growth_percent"` // Рост капитала в %
	MonthlySharpe           float64            `json:"monthly_sharpe"`        // Месячный коэф. Шарпа
	MonthlyReturns          map[string]float64 `json:"monthly_returns"`
	AverageGainPercent      float64            `json:"average_gain_percent"`
	AverageLossPercent      float64            `json:"average_loss_percent"`
	TotalReturnPercent      float64            `json:"total_return_percent"`
}

// Client is a wrapper for HTTP client with rate limiting

// Структура для управления риском
type PositionSizingResult struct {
	PositionSize    float64 `json:"position_size"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
	AccountRisk     float64 `json:"account_risk"`
}

type Anomaly struct {
	Type      string
	Timestamp string
	Severity  float64
	Details   string
}

// Signal - структура для торговых сигналов
type Signal struct {
	Type       string  // buy, sell, exit
	Strength   float64 // 0-1
	Timestamp  string
	Source     string // indicator, pattern, combination
	Indicators map[string]float64
}

// Payment status constants
const (
	PaymentStatusPending  = "pending"
	PaymentStatusAccepted = "accepted"
	PaymentStatusClosed   = "closed"
)

// UserSubscription represents a user's subscription status
type UserSubscription struct {
	UserID        int64     `json:"user_id"`
	ChatID        int64     `json:"chat_id"`
	Status        string    `json:"status"` // pending, accepted, closed
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`    // when the subscription expires
	PaymentID     string    `json:"payment_id"`    // Stripe payment ID
	CurrencyPair  string    `json:"currency_pair"` // Selected currency pair
	Timeframe     string    `json:"timeframe"`     // Selected timeframe
	LastPredicted time.Time `json:"last_predicted,omitempty"`
}
