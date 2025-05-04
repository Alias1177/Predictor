package anomaly

import (
	"chi/Predictor/internal/calculate"
	"chi/Predictor/models"
	"math"
)

func EnhancedMarketRegimeClassification(candles []models.Candle) *models.MarketRegime {
	if len(candles) < 20 {
		return &models.MarketRegime{
			Type:             "UNKNOWN",
			Strength:         0,
			Direction:        "NEUTRAL",
			VolatilityLevel:  "NORMAL",
			MomentumStrength: 0,
			LiquidityRating:  "NORMAL",
			PriceStructure:   "UNKNOWN",
		}
	}

	// Initialize market regime
	regime := &models.MarketRegime{
		Type:             "UNKNOWN",
		Strength:         0,
		Direction:        "NEUTRAL",
		VolatilityLevel:  "NORMAL",
		MomentumStrength: 0,
		LiquidityRating:  "NORMAL",
		PriceStructure:   "UNKNOWN",
	}

	// Calculate key indicators
	adx, plusDI, minusDI := calculate.CalculateADX(candles, 14)
	atr10 := calculate.CalculateATR(candles, 10)
	atr30 := calculate.CalculateATR(candles, 30)

	// Volatility analysis
	volatilityRatio := atr10 / atr30
	if volatilityRatio > 1.5 {
		regime.VolatilityLevel = "HIGH"
	} else if volatilityRatio < 0.7 {
		regime.VolatilityLevel = "LOW"
	}

	// Momentum analysis
	var momentumScore float64
	current := candles[len(candles)-1].Close
	prev5 := candles[len(candles)-6].Close
	prev10 := candles[len(candles)-11].Close

	// Get index for prev20 carefully to avoid out of range error
	prev20Idx := len(candles) - 21
	var prev20 float64
	if prev20Idx >= 0 {
		prev20 = candles[prev20Idx].Close
	} else {
		// Fall back to earliest available candle
		prev20 = candles[0].Close
	}

	// Weight shorter term changes more heavily
	momentum5 := (current - prev5) / prev5
	momentum10 := (current - prev10) / prev10
	momentum20 := (current - prev20) / prev20

	// Weighted momentum score
	momentumScore = (momentum5 * 0.5) + (momentum10 * 0.3) + (momentum20 * 0.2)
	regime.MomentumStrength = math.Min(math.Abs(momentumScore)*10, 1.0)

	if momentumScore > 0 {
		regime.Direction = "BULLISH"
	} else if momentumScore < 0 {
		regime.Direction = "BEARISH"
	}

	// ADX-based trend analysis
	if adx > 25 {
		if plusDI > minusDI {
			regime.Type = "TRENDING"
			regime.PriceStructure = "TRENDING_UP"
		} else {
			regime.Type = "TRENDING"
			regime.PriceStructure = "TRENDING_DOWN"
		}

		// Calculate trend strength
		regime.Strength = math.Min(adx/50.0, 1.0)
	} else {
		// Check for range-bound market
		highestHigh := 0.0
		lowestLow := 999999.0

		for i := len(candles) - 20; i < len(candles); i++ {
			if candles[i].High > highestHigh {
				highestHigh = candles[i].High
			}
			if candles[i].Low < lowestLow {
				lowestLow = candles[i].Low
			}
		}

		rangeHeight := highestHigh - lowestLow
		normalizedRange := rangeHeight / atr10

		if normalizedRange < 5.0 {
			regime.Type = "RANGING"
			regime.PriceStructure = "RANGE_BOUND"

			// Strength inversely related to ADX
			regime.Strength = math.Min((30.0-adx)/30.0, 1.0)
		} else {
			// Check for choppy market conditions
			var directionalChanges int
			prevDirection := candles[len(candles)-20].Close > candles[len(candles)-21].Close

			for i := len(candles) - 19; i < len(candles); i++ {
				currentDirection := candles[i].Close > candles[i-1].Close
				if currentDirection != prevDirection {
					directionalChanges++
					prevDirection = currentDirection
				}
			}

			if directionalChanges > 8 {
				regime.Type = "CHOPPY"
				regime.Strength = math.Min(float64(directionalChanges)/15.0, 1.0)
			} else if volatilityRatio > 1.8 {
				regime.Type = "VOLATILE"
				regime.Strength = math.Min(volatilityRatio/3.0, 1.0)

				if momentumScore > 0.02 {
					regime.PriceStructure = "BREAKOUT"
				} else if momentumScore < -0.02 {
					regime.PriceStructure = "BREAKDOWN"
				}
			} else {
				// Default to mild trending
				regime.Type = "TRENDING"
				regime.Strength = math.Min(adx/30.0, 0.7) // Cap at 0.7 for mild trends

				if plusDI > minusDI {
					regime.PriceStructure = "TRENDING_UP"
				} else {
					regime.PriceStructure = "TRENDING_DOWN"
				}
			}
		}
	}

	// Liquidity analysis based on average true range relative to price
	avgPrice := (current + prev5 + prev10) / 3
	liquidityRatio := atr10 / avgPrice

	if liquidityRatio > 0.005 { // 0.5% volatility relative to price
		regime.LiquidityRating = "LOW"
	} else if liquidityRatio < 0.001 { // 0.1% volatility relative to price
		regime.LiquidityRating = "HIGH"
	}

	return regime
}
