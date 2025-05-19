package models

// TradingSuggestion представляет торговую рекомендацию
type TradingSuggestion struct {
	Action          string
	Direction       string
	Confidence      string
	Score           float64
	EntryPrice      float64
	StopLoss        float64
	TakeProfit      float64
	PositionSize    float64
	RiskRewardRatio float64
	AccountRisk     float64
	Factors         []string
}
