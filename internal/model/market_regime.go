package model

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
