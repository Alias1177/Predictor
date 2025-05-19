package models

// TechnicalIndicators содержит все технические индикаторы
type TechnicalIndicators struct {
	RSI              float64
	MACD             float64
	MACDHist         float64
	MACDSignal       float64
	BBUpper          float64
	BBMiddle         float64
	BBLower          float64
	EMA              float64
	ADX              float64
	PlusDI           float64
	MinusDI          float64
	ATR              float64
	Stochastic       float64
	StochasticSignal float64
	TradeSignal      string
	Support          []float64
	Resistance       []float64
}
