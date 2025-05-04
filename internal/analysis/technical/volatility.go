package technical

import (
	"chi/Predictor/internal/model"
	"math"
)

// CalculateBollingerBands calculates Bollinger Bands
func CalculateBollingerBands(candles []model.Candle, period int, stdDev float64) (float64, float64, float64) {
	if len(candles) < period {
		last := candles[len(candles)-1].Close
		return last, last, last // Return last close if not enough data
	}

	// Calculate SMA
	var sum float64
	for i := len(candles) - period; i < len(candles); i++ {
		sum += candles[i].Close
	}
	middle := sum / float64(period)

	// Calculate standard deviation
	var variance float64
	for i := len(candles) - period; i < len(candles); i++ {
		variance += math.Pow(candles[i].Close-middle, 2)
	}
	sd := math.Sqrt(variance / float64(period))

	upper := middle + (sd * stdDev)
	lower := middle - (sd * stdDev)

	return upper, middle, lower
}

// CalculateATR calculates Average True Range
func CalculateATR(candles []model.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}

	var trueRanges []float64

	// Calculate True Range for each candle
	for i := 1; i < len(candles); i++ {
		// True Range is the greatest of:
		// 1. Current High - Current Low
		// 2. Abs(Current High - Previous Close)
		// 3. Abs(Current Low - Previous Close)
		highLow := candles[i].High - candles[i].Low
		highPrevClose := math.Abs(candles[i].High - candles[i-1].Close)
		lowPrevClose := math.Abs(candles[i].Low - candles[i-1].Close)

		trueRange := math.Max(highLow, math.Max(highPrevClose, lowPrevClose))
		trueRanges = append(trueRanges, trueRange)
	}

	// If we don't have enough data for the period, use what we have
	periodToUse := period
	if len(trueRanges) < period {
		periodToUse = len(trueRanges)
	}

	// Calculate average of true ranges
	var sum float64
	for i := len(trueRanges) - periodToUse; i < len(trueRanges); i++ {
		sum += trueRanges[i]
	}

	return sum / float64(periodToUse)
}

// AssessVolatilityConditions analyzes market volatility
func AssessVolatilityConditions(candles []model.Candle) (string, float64) {
	// Calculate ATR for different periods
	atr5 := CalculateATR(candles, 5)
	atr20 := CalculateATR(candles, 20)

	// Calculate volatility ratio
	volatilityRatio := atr5 / atr20

	// Determine volatility regime
	volatilityRegime := "NORMAL"
	if volatilityRatio > 1.5 {
		volatilityRegime = "HIGH"
	} else if volatilityRatio < 0.7 {
		volatilityRegime = "LOW"
	}

	// Calculate recent range
	var highestHigh, lowestLow float64
	for i := len(candles) - 10; i < len(candles); i++ {
		if i == len(candles)-10 || candles[i].High > highestHigh {
			highestHigh = candles[i].High
		}
		if i == len(candles)-10 || candles[i].Low < lowestLow {
			lowestLow = candles[i].Low
		}
	}

	// Calculate expected move for the selected timeframe
	expectedMove := atr5

	return volatilityRegime, expectedMove
}

// CalculateVolatilityRatio calculates the ratio between short-term and long-term volatility
func CalculateVolatilityRatio(candles []model.Candle, shortPeriod, longPeriod int) float64 {
	if len(candles) < longPeriod+1 {
		return 1.0 // Default neutral value if not enough data
	}

	atrShort := CalculateATR(candles, shortPeriod)
	atrLong := CalculateATR(candles, longPeriod)

	if atrLong == 0 {
		return 1.0
	}

	return atrShort / atrLong
}

// IdentifySupportResistance finds potential support and resistance levels
func IdentifySupportResistance(candles []model.Candle) ([]float64, []float64) {
	if len(candles) < 20 {
		return nil, nil
	}

	// Prepare price points map to track touch frequency
	pricePoints := make(map[float64]int)
	priceTolerance := 0.0002 // For EUR/USD, approximately 2 pips

	// Scan for swing highs and lows
	for i := 2; i < len(candles)-2; i++ {
		// Potential support (swing low)
		if candles[i].Low < candles[i-1].Low &&
			candles[i].Low < candles[i-2].Low &&
			candles[i].Low < candles[i+1].Low &&
			candles[i].Low < candles[i+2].Low {

			// Round to nearby level for clustering
			level := math.Round(candles[i].Low/priceTolerance) * priceTolerance
			pricePoints[level]++
		}

		// Potential resistance (swing high)
		if candles[i].High > candles[i-1].High &&
			candles[i].High > candles[i-2].High &&
			candles[i].High > candles[i+1].High &&
			candles[i].High > candles[i+2].High {

			// Round to nearby level for clustering
			level := math.Round(candles[i].High/priceTolerance) * priceTolerance
			pricePoints[level]++
		}
	}

	// Check for recent closes near these levels
	for i := len(candles) - 10; i < len(candles); i++ {
		for price := range pricePoints {
			// Check if close is near this level
			if math.Abs(candles[i].Close-price) < priceTolerance*2 {
				pricePoints[price]++
			}
		}
	}

	// Process and sort levels by strength
	type PriceLevel struct {
		Price    float64
		Strength int
	}

	var levels []PriceLevel
	for price, strength := range pricePoints {
		levels = append(levels, PriceLevel{Price: price, Strength: strength})
	}

	// Sort by strength (descending)
	sortLevelsByStrength(levels)

	// Current price
	currentPrice := candles[len(candles)-1].Close

	// Separate into support and resistance
	var support, resistance []float64
	for _, level := range levels {
		if level.Price < currentPrice {
			support = append(support, level.Price)
		} else if level.Price > currentPrice {
			resistance = append(resistance, level.Price)
		}
	}

	// Sort support (descending - nearest first) and resistance (ascending - nearest first)
	sortSupportLevels(support)
	sortResistanceLevels(resistance, currentPrice)

	// Limit to most significant levels
	maxLevels := 3
	if len(support) > maxLevels {
		support = support[:maxLevels]
	}
	if len(resistance) > maxLevels {
		resistance = resistance[:maxLevels]
	}

	return support, resistance
}

// Helper functions for sorting
func sortLevelsByStrength(levels []PriceLevel) {
	// Sort by strength (descending)
	for i := 0; i < len(levels)-1; i++ {
		for j := i + 1; j < len(levels); j++ {
			if levels[i].Strength < levels[j].Strength {
				levels[i], levels[j] = levels[j], levels[i]
			}
		}
	}
}

func sortSupportLevels(support []float64) {
	// Sort support (descending - nearest first)
	for i := 0; i < len(support)-1; i++ {
		for j := i + 1; j < len(support); j++ {
			if support[i] < support[j] {
				support[i], support[j] = support[j], support[i]
			}
		}
	}
}

func sortResistanceLevels(resistance []float64, currentPrice float64) {
	// Sort resistance (ascending - nearest first)
	for i := 0; i < len(resistance)-1; i++ {
		for j := i + 1; j < len(resistance); j++ {
			if resistance[i] > resistance[j] {
				resistance[i], resistance[j] = resistance[j], resistance[i]
			}
		}
	}
}
