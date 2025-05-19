package models

// MarketRegime представляет текущий рыночный режим
type MarketRegime struct {
	Type            string
	Direction       string
	Strength        float64
	VolatilityLevel string
}
