package model

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
