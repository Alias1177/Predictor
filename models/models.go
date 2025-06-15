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
	Symbol    string    `json:"symbol"`    // Символ инструмента
	TimeFrame string    `json:"timeframe"` // Таймфрейм
	Open      float64   `json:"open"`      // Цена открытия
	High      float64   `json:"high"`      // Максимальная цена
	Low       float64   `json:"low"`       // Минимальная цена
	Close     float64   `json:"close"`     // Цена закрытия
	Volume    int64     `json:"volume"`    // Объем
	Timestamp time.Time `json:"timestamp"` // Временная метка
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

	DivergenceStats struct {
		BullishCorrect   int `json:"bullish_correct"`
		BullishIncorrect int `json:"bullish_incorrect"`
		BearishCorrect   int `json:"bearish_correct"`
		BearishIncorrect int `json:"bearish_incorrect"`
	} `json:"divergence_stats"`
}

// Client is a wrapper for HTTP client with rate limiting

// Структура для управления риском
type PositionSizingResult struct {
	PositionSize      float64            `json:"position_size"`
	StopLoss          float64            `json:"stop_loss"`
	TakeProfit        float64            `json:"take_profit"`
	RiskRewardRatio   float64            `json:"risk_reward_ratio"`
	AccountRisk       float64            `json:"account_risk"`
	AdditionalTargets map[string]float64 `json:"additional_targets,omitempty"`
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
	UserID               int64     `json:"user_id"`
	ChatID               int64     `json:"chat_id"`
	Status               string    `json:"status"` // pending, accepted, closed
	CreatedAt            time.Time `json:"created_at"`
	ExpiresAt            time.Time `json:"expires_at"`             // when the subscription expires
	PaymentID            string    `json:"payment_id"`             // Stripe payment ID
	StripeSubscriptionID string    `json:"stripe_subscription_id"` // Stripe subscription ID for cancellation
	CurrencyPair         string    `json:"currency_pair"`          // Selected currency pair
	Timeframe            string    `json:"timeframe"`              // Selected timeframe
	LastPredicted        time.Time `json:"last_predicted,omitempty"`
}

// OrderFlow представляет анализ потока ордеров
type OrderFlow struct {
	Direction       string  `json:"direction"`
	Strength        float64 `json:"strength"`
	BuyingPressure  float64 `json:"buying_pressure"`
	SellingPressure float64 `json:"selling_pressure"`
	DeltaPercentage float64 `json:"delta_percentage"`
	IsClimaxVolume  bool    `json:"is_climax_volume"`
	IsExhaustion    bool    `json:"is_exhaustion"`
}

// PatternPoint представляет точку в гармоническом паттерне
type PatternPoint struct {
	Index int     `json:"index"`
	Price float64 `json:"price"`
}

// HarmonicPattern представляет гармонический паттерн
type HarmonicPattern struct {
	Type              string                  `json:"type"`
	Direction         string                  `json:"direction"`
	Points            map[string]PatternPoint `json:"points"`
	Ratios            map[string]float64      `json:"ratios"`
	CompletionIndex   int                     `json:"completion_index"`
	PotentialReversal bool                    `json:"potential_reversal"`
}

// SimulationResult представляет результат одной симуляции Монте-Карло
type SimulationResult struct {
	FinalBalance float64   `json:"final_balance"`
	TotalReturn  float64   `json:"total_return"`
	MaxDrawdown  float64   `json:"max_drawdown"`
	EquityCurve  []float64 `json:"equity_curve,omitempty"`
}

// MonteCarloPercentiles представляет процентили результатов симуляций
type MonteCarloPercentiles struct {
	Worst  float64 `json:"worst"`
	P10    float64 `json:"p10"`
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	P75    float64 `json:"p75"`
	P90    float64 `json:"p90"`
	Best   float64 `json:"best"`
}

// MonteCarloResults представляет общие результаты симуляции Монте-Карло
type MonteCarloResults struct {
	Simulations         int                   `json:"simulations"`
	Returns             MonteCarloPercentiles `json:"returns"`
	AverageDrawdown     float64               `json:"average_drawdown"`
	WorstDrawdown       float64               `json:"worst_drawdown"`
	ProbabilityOfProfit float64               `json:"probability_of_profit"`
}

// DivergencePoint представляет точку в дивергенции
type DivergencePoint struct {
	Index int     `json:"index"`
	Value float64 `json:"value"`
}

// Divergence представляет дивергенцию между ценой и индикатором
type Divergence struct {
	Type            string            `json:"type"`      // REGULAR или HIDDEN
	Direction       string            `json:"direction"` // BULLISH или BEARISH
	PricePoints     []DivergencePoint `json:"price_points"`
	IndicatorPoints []DivergencePoint `json:"indicator_points"`
	Indicator       string            `json:"indicator"`       // Какой индикатор (RSI, MACD и т.д.)
	SignalStrength  float64           `json:"signal_strength"` // Сила сигнала от 0 до 1
}

// TradingSuggestion содержит конкретные рекомендации по торговле
type TradingSuggestion struct {
	Action          string   `json:"action"`            // BUY, SELL, NO_TRADE
	Direction       string   `json:"direction"`         // UP, DOWN, NEUTRAL
	Confidence      string   `json:"confidence"`        // HIGH, MEDIUM, LOW
	Score           float64  `json:"score"`             // Числовой показатель уверенности
	EntryPrice      float64  `json:"entry_price"`       // Рекомендуемая цена входа
	StopLoss        float64  `json:"stop_loss"`         // Уровень стоп-лосса
	TakeProfit      float64  `json:"take_profit"`       // Уровень тейк-профита
	PositionSize    float64  `json:"position_size"`     // Рекомендуемый размер позиции
	RiskRewardRatio float64  `json:"risk_reward_ratio"` // Соотношение риск/доходность
	AccountRisk     float64  `json:"account_risk"`      // Процент риска от размера счета
	Factors         []string `json:"factors"`           // Факторы, повлиявшие на решение
}

// Prediction представляет результат анализа и предсказания
type Prediction struct {
	Direction         string
	Confidence        string
	Score             float64
	Factors           []string
	TradingSuggestion *TradingSuggestion
}

// MarketAnalysis представляет полный анализ рынка
type MarketAnalysis struct {
	Symbol          string                  `json:"symbol"`
	TimeFrame       string                  `json:"timeframe"`
	LastPrice       float64                 `json:"last_price"`
	LastUpdated     time.Time               `json:"last_updated"`
	Direction       string                  `json:"direction"`
	Confidence      float64                 `json:"confidence"`
	MarketRegime    string                  `json:"market_regime"`
	RegimeStrength  float64                 `json:"regime_strength"`
	Volatility      string                  `json:"volatility"`
	MarketSentiment *MarketSentiment        `json:"market_sentiment"`
	Correlations    *CorrelationAnalysis    `json:"correlations"`
	Liquidity       *LiquidityAnalysis      `json:"liquidity"`
	Volume          *VolumeAnalysis         `json:"volume"`
	Microstructure  *MicrostructureAnalysis `json:"microstructure"`
	News            *NewsAnalysis           `json:"news"`
	Fundamentals    *FundamentalAnalysis    `json:"fundamentals"`
}

// MarketSentiment представляет настроения рынка
type MarketSentiment struct {
	FearGreedIndex  float64 `json:"fear_greed_index"` // 0-100
	MarketMood      string  `json:"market_mood"`      // bullish/bearish/neutral
	SocialSentiment float64 `json:"social_sentiment"` // -1 до 1
	VolumeSentiment float64 `json:"volume_sentiment"` // -1 до 1
	VolatilityMood  string  `json:"volatility_mood"`  // high/low/medium
}

// CorrelationAnalysis представляет результаты анализа корреляций
type CorrelationAnalysis struct {
	AssetCorrelations     map[string]float64 `json:"asset_correlations"`     // Корреляции с другими активами
	MarketCorrelation     float64            `json:"market_correlation"`     // Корреляция с рынком
	VolatilityCorrelation float64            `json:"volatility_correlation"` // Корреляция с волатильностью
	VolumeCorrelation     float64            `json:"volume_correlation"`     // Корреляция с объемом
}

// LiquidityAnalysis представляет результаты анализа ликвидности
type LiquidityAnalysis struct {
	BidAskSpread   float64 `json:"bid_ask_spread"`   // Спред между bid и ask
	OrderBookDepth float64 `json:"order_book_depth"` // Глубина стакана
	VolumeProfile  float64 `json:"volume_profile"`   // Профиль объема
	MarketImpact   float64 `json:"market_impact"`    // Влияние на рынок
	LiquidityScore float64 `json:"liquidity_score"`  // Общий скор ликвидности
}

// VolumeAnalysis представляет результаты анализа объемов
type VolumeAnalysis struct {
	VolumeTrend     string          `json:"volume_trend"`     // Тренд объема
	VolumeStrength  float64         `json:"volume_strength"`  // Сила объема
	VolumeProfile   float64         `json:"volume_profile"`   // Профиль объема
	VolumeImbalance float64         `json:"volume_imbalance"` // Дисбаланс объема
	VolumeClusters  []VolumeCluster `json:"volume_clusters"`  // Кластеры объема
}

// VolumeCluster представляет кластер объема
type VolumeCluster struct {
	Price     float64 `json:"price"`     // Цена кластера
	Volume    int64   `json:"volume"`    // Объем кластера
	Direction string  `json:"direction"` // Направление (buy/sell)
}

// MicrostructureAnalysis представляет результаты анализа микроструктуры
type MicrostructureAnalysis struct {
	OrderFlow       OrderFlowAnalysis      `json:"order_flow"`       // Анализ потока ордеров
	PriceImpact     PriceImpactAnalysis    `json:"price_impact"`     // Анализ влияния на цену
	MarketQuality   MarketQualityMetrics   `json:"market_quality"`   // Метрики качества рынка
	TradingActivity TradingActivityMetrics `json:"trading_activity"` // Метрики торговой активности
}

// OrderFlowAnalysis представляет анализ потока ордеров
type OrderFlowAnalysis struct {
	BuyPressure   float64 `json:"buy_pressure"`   // Давление покупателей
	SellPressure  float64 `json:"sell_pressure"`  // Давление продавцов
	NetFlow       float64 `json:"net_flow"`       // Чистый поток
	FlowImbalance float64 `json:"flow_imbalance"` // Дисбаланс потока
	LargeOrders   int     `json:"large_orders"`   // Количество крупных ордеров
}

// PriceImpactAnalysis представляет анализ влияния на цену
type PriceImpactAnalysis struct {
	ImmediateImpact float64 `json:"immediate_impact"` // Мгновенное влияние
	PermanentImpact float64 `json:"permanent_impact"` // Постоянное влияние
	ImpactDecay     float64 `json:"impact_decay"`     // Затухание влияния
	PriceElasticity float64 `json:"price_elasticity"` // Эластичность цены
}

// MarketQualityMetrics представляет метрики качества рынка
type MarketQualityMetrics struct {
	Efficiency    float64 `json:"efficiency"`    // Эффективность рынка
	Resilience    float64 `json:"resilience"`    // Устойчивость рынка
	Fragmentation float64 `json:"fragmentation"` // Фрагментация рынка
	Transparency  float64 `json:"transparency"`  // Прозрачность рынка
}

// TradingActivityMetrics представляет метрики торговой активности
type TradingActivityMetrics struct {
	TradeFrequency float64 `json:"trade_frequency"` // Частота сделок
	TradeSize      float64 `json:"trade_size"`      // Размер сделок
	TradeValue     float64 `json:"trade_value"`     // Стоимость сделок
	ActiveTraders  int     `json:"active_traders"`  // Активные трейдеры
}

// NewsAnalysis представляет анализ новостей
type NewsAnalysis struct {
	Sentiment    float64            `json:"sentiment"`     // Общий сентимент (-1 до 1)
	Impact       float64            `json:"impact"`        // Влияние на рынок (0-1)
	Relevance    float64            `json:"relevance"`     // Релевантность (0-1)
	NewsCount    int                `json:"news_count"`    // Количество новостей
	TopNews      []NewsItem         `json:"top_news"`      // Важные новости
	MarketImpact map[string]float64 `json:"market_impact"` // Влияние на разные аспекты рынка
}

// NewsItem представляет отдельную новость
type NewsItem struct {
	Title       string    `json:"title"`        // Заголовок
	Content     string    `json:"content"`      // Содержание
	Sentiment   float64   `json:"sentiment"`    // Сентимент (-1 до 1)
	Impact      float64   `json:"impact"`       // Влияние (0-1)
	PublishedAt time.Time `json:"published_at"` // Время публикации
	Symbol      string    `json:"symbol"`       // Символ инструмента
}

// FundamentalAnalysis представляет фундаментальный анализ
type FundamentalAnalysis struct {
	EconomicIndicators map[string]float64 `json:"economic_indicators"` // Экономические индикаторы
	MarketConditions   map[string]float64 `json:"market_conditions"`   // Рыночные условия
	RiskFactors        map[string]float64 `json:"risk_factors"`        // Факторы риска
	MarketRegime       string             `json:"market_regime"`       // Режим рынка
	RegimeStrength     float64            `json:"regime_strength"`     // Сила режима (0-1)
}

// PromoCode represents a promotional code for subscription discounts
type PromoCode struct {
	ID              int64     `json:"id"`
	Code            string    `json:"code"`                      // Промокод
	DiscountPercent int       `json:"discount_percent"`          // Скидка в процентах (0-100)
	DiscountAmount  float64   `json:"discount_amount,omitempty"` // Фиксированная скидка в долларах
	IsPercentage    bool      `json:"is_percentage"`             // true - процентная скидка, false - фиксированная
	UsageLimit      int       `json:"usage_limit"`               // Максимальное количество использований (-1 = неограничено)
	UsedCount       int       `json:"used_count"`                // Количество использований
	ExpiresAt       time.Time `json:"expires_at,omitempty"`      // Дата истечения (пустая = не истекает)
	CreatedAt       time.Time `json:"created_at"`
	IsActive        bool      `json:"is_active"`             // Активен ли промокод
	CreatedBy       string    `json:"created_by,omitempty"`  // Кто создал промокод
	Description     string    `json:"description,omitempty"` // Описание промокода
}

// PromoCodeUsage represents usage history of promo codes
type PromoCodeUsage struct {
	ID          int64     `json:"id"`
	PromoCodeID int64     `json:"promo_code_id"`
	UserID      int64     `json:"user_id"`
	UsedAt      time.Time `json:"used_at"`
	PaymentID   string    `json:"payment_id,omitempty"` // Stripe payment ID
	SavedAmount float64   `json:"saved_amount"`         // Сколько пользователь сэкономил
}
