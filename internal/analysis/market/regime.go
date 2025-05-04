package market

import (
	"chi/Predictor/internal/analysis/technical"
	"chi/Predictor/internal/model"
	"math"
)

// ClassifyMarketRegime provides a detailed market regime analysis
func ClassifyMarketRegime(candles []model.Candle) *model.MarketRegime {
	if len(candles) < 20 {
		return &model.MarketRegime{
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
	regime := &model.MarketRegime{
		Type:             "UNKNOWN",
		Strength:         0,
		Direction:        "NEUTRAL",
		VolatilityLevel:  "NORMAL",
		MomentumStrength: 0,
		LiquidityRating:  "NORMAL",
		PriceStructure:   "UNKNOWN",
	}

	// Calculate key indicators
	adx, plusDI, minusDI := technical.CalculateADX(candles, 14)
	atr10 := technical.CalculateATR(candles, 10)
	atr30 := technical.CalculateATR(candles, 30)

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

// DetectTrendAlignment identifies trend alignment across timeframes
func DetectTrendAlignment(mtfData map[string][]model.Candle) (string, float64) {
	// Calculate EMAs for each timeframe
	trendScores := make(map[string]float64)

	for timeframe, candles := range mtfData {
		if len(candles) < 3 {
			continue
		}

		// Get last 3 closes
		last := candles[len(candles)-1].Close
		prev := candles[len(candles)-2].Close
		prevPrev := candles[len(candles)-3].Close

		// Calculate fast and slow EMAs
		fastEMA := technical.CalculateEMA(candles, 8)
		slowEMA := technical.CalculateEMA(candles, 21)

		// Determine trend direction and strength
		trendDirection := 0.0
		if last > prev && prev > prevPrev {
			trendDirection = 1.0 // Uptrend
		} else if last < prev && prev < prevPrev {
			trendDirection = -1.0 // Downtrend
		}

		// EMA alignment factor
		emaAlignment := 0.0
		if fastEMA > slowEMA {
			emaAlignment = 1.0
		} else if fastEMA < slowEMA {
			emaAlignment = -1.0
		}

		// Price position relative to EMAs
		pricePosition := 0.0
		if last > fastEMA && last > slowEMA {
			pricePosition = 1.0
		} else if last < fastEMA && last < slowEMA {
			pricePosition = -1.0
		}

		// Weight different timeframes (higher timeframes have more weight)
		weight := 1.0
		if timeframe == "5min" {
			weight = 2.0
		} else if timeframe == "15min" {
			weight = 3.0
		}

		// Combined score for this timeframe
		trendScores[timeframe] = (trendDirection + emaAlignment + pricePosition) * weight
	}

	// Calculate overall alignment score
	var totalScore float64
	for _, score := range trendScores {
		totalScore += score
	}

	// Normalize score to range [-1, 1]
	maxPossibleScore := 6.0 // 3 metrics * max weight of 2
	if len(trendScores) > 0 {
		alignmentScore := totalScore / maxPossibleScore

		// Determine alignment direction
		alignmentDirection := "NEUTRAL"
		alignmentStrength := math.Abs(alignmentScore)

		if alignmentScore > 0.3 {
			alignmentDirection = "BULLISH"
		} else if alignmentScore < -0.3 {
			alignmentDirection = "BEARISH"
		}

		return alignmentDirection, alignmentStrength
	}

	return "NEUTRAL", 0.0
}

// AdaptIndicatorParameters dynamically adjusts indicator parameters based on market conditions
func AdaptIndicatorParameters(candles []model.Candle, config map[string]interface{}) map[string]interface{} {
	if candles == nil || len(candles) < 30 {
		return config // Return original config if not enough data
	}

	// Create a copy of the config
	adaptedConfig := make(map[string]interface{})
	for k, v := range config {
		adaptedConfig[k] = v
	}

	// Calculate volatility measures
	atr5 := technical.CalculateATR(candles, 5)
	atr20 := technical.CalculateATR(candles, 20)
	volatilityRatio := atr5 / atr20

	// Check market regime
	regime := ClassifyMarketRegime(candles)

	// Adjust RSI period based on volatility
	if rsiPeriod, ok := adaptedConfig["RSIPeriod"].(int); ok {
		if volatilityRatio > 1.5 {
			// Higher volatility - use shorter periods for faster reaction
			adaptedConfig["RSIPeriod"] = max(5, rsiPeriod-2)
		} else if volatilityRatio < 0.7 {
			// Lower volatility - use longer periods to reduce noise
			adaptedConfig["RSIPeriod"] = min(14, rsiPeriod+2)
		}
	}

	// Adjust Bollinger Bands parameters based on market regime
	if bbStdDev, ok := adaptedConfig["BBStdDev"].(float64); ok {
		if regime.Type == "TRENDING" && regime.Strength > 0.7 {
			// In strong trends, widen the bands
			adaptedConfig["BBStdDev"] = math.Min(3.0, bbStdDev+0.3)
		} else if regime.Type == "RANGING" {
			// In range-bound markets, tighten the bands
			adaptedConfig["BBStdDev"] = math.Max(1.8, bbStdDev-0.3)
		}
	}

	// Adjust MACD parameters based on market regime
	if macdFastPeriod, ok := adaptedConfig["MACDFastPeriod"].(int); ok {
		if macdSlowPeriod, ok := adaptedConfig["MACDSlowPeriod"].(int); ok {
			if regime.Type == "CHOPPY" || regime.Type == "RANGING" {
				// In non-trending markets, use wider MACD settings
				adaptedConfig["MACDFastPeriod"] = min(12, macdFastPeriod+2)
				adaptedConfig["MACDSlowPeriod"] = min(26, macdSlowPeriod+3)
			} else if regime.Type == "TRENDING" && regime.Strength > 0.6 {
				// In trending markets, use more responsive MACD settings
				adaptedConfig["MACDFastPeriod"] = max(5, macdFastPeriod-1)
				adaptedConfig["MACDSlowPeriod"] = max(12, macdSlowPeriod-2)
			}
		}
	}

	// Adjust EMA period based on momentum
	if emaPeriod, ok := adaptedConfig["EMAPeriod"].(int); ok {
		if regime.MomentumStrength > 0.7 {
			// Strong momentum - use shorter EMA period
			adaptedConfig["EMAPeriod"] = max(8, emaPeriod-2)
		} else if regime.MomentumStrength < 0.3 {
			// Weak momentum - use longer EMA period
			adaptedConfig["EMAPeriod"] = min(15, emaPeriod+2)
		}
	}

	return adaptedConfig
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
