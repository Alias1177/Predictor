package analyze

import (
	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
)

// analyzeOrderFlow analyzes volume dynamics
func analyzeOrderFlow(candles []models.Candle) (string, float64) {
	// Check if we have volume data
	hasVolume := true
	for _, c := range candles {
		if c.Volume == 0 {
			hasVolume = false
			break
		}
	}

	if !hasVolume {
		return "NO_VOLUME_DATA", 0
	}

	// Calculate volume weighted price
	var totalVolume int64
	var volumeWeightedPrice float64

	for i := len(candles) - 5; i < len(candles); i++ {
		volumeWeightedPrice += candles[i].Close * float64(candles[i].Volume)
		totalVolume += candles[i].Volume
	}

	if totalVolume > 0 {
		volumeWeightedPrice /= float64(totalVolume)
	}

	// Calculate volume trend
	var upVolume, downVolume int64
	for i := len(candles) - 5; i < len(candles); i++ {
		if candles[i].Close > candles[i].Open {
			upVolume += candles[i].Volume
		} else {
			downVolume += candles[i].Volume
		}
	}

	// Calculate volume ratio
	volumeRatio := 0.5
	if upVolume+downVolume > 0 {
		volumeRatio = float64(upVolume) / float64(upVolume+downVolume)
	}

	// Determine volume flow direction
	flowDirection := "NEUTRAL"
	if volumeRatio > 0.65 {
		flowDirection = "BULLISH"
	} else if volumeRatio < 0.35 {
		flowDirection = "BEARISH"
	}

	return flowDirection, volumeWeightedPrice
}

// assessVolatilityConditions analyzes market volatility
func assessVolatilityConditions(candles []models.Candle) (string, float64) {
	// Calculate ATR for different periods
	atr5 := utils.CalculateATR(candles, 5)
	atr20 := utils.CalculateATR(candles, 20)

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
