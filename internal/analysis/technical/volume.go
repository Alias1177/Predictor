package technical

import (
	"chi/Predictor/internal/model"
	"math"
)

// CalculateOBV calculates On-Balance Volume
func CalculateOBV(candles []model.Candle) float64 {
	if len(candles) < 2 {
		return 0.0
	}

	// Check if we have volume data
	if candles[len(candles)-1].Volume == 0 {
		return 0.0 // No volume data available
	}

	// Initialize OBV with the first available volume
	obv := float64(candles[0].Volume)

	// Calculate OBV
	for i := 1; i < len(candles); i++ {
		if candles[i].Close > candles[i-1].Close {
			// Price up, add volume
			obv += float64(candles[i].Volume)
		} else if candles[i].Close < candles[i-1].Close {
			// Price down, subtract volume
			obv -= float64(candles[i].Volume)
		}
		// If price unchanged, OBV remains the same
	}

	return obv
}

// AnalyzeOrderFlow analyzes volume dynamics
func AnalyzeOrderFlow(candles []model.Candle) (string, float64) {
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

// CalculateVolumeChange calculates volume change percentage over a specific period
func CalculateVolumeChange(candles []model.Candle, period int) float64 {
	if len(candles) < period+1 || candles[len(candles)-period-1].Volume == 0 {
		return 0.0 // Not enough data or zero volume in the base period
	}

	// Check if volume data is available
	hasVolume := true
	for i := len(candles) - period - 1; i < len(candles); i++ {
		if candles[i].Volume == 0 {
			hasVolume = false
			break
		}
	}

	if !hasVolume {
		return 0.0
	}

	// Calculate volume change
	firstVolume := float64(candles[len(candles)-period-1].Volume)
	lastVolume := float64(candles[len(candles)-1].Volume)

	return ((lastVolume - firstVolume) / firstVolume) * 100
}

// CalculateAverageVolume calculates the average volume over a specific period
func CalculateAverageVolume(candles []model.Candle, period int) float64 {
	if len(candles) < period {
		return 0.0
	}

	var totalVolume int64
	for i := len(candles) - period; i < len(candles); i++ {
		totalVolume += candles[i].Volume
	}

	return float64(totalVolume) / float64(period)
}

// DetectVolumeDivergence detects divergence between price and volume
// Returns true if divergence is detected, with type and strength
func DetectVolumeDivergence(candles []model.Candle, period int) (bool, string, float64) {
	if len(candles) < period+1 {
		return false, "", 0.0
	}

	// Check if volume data is available
	hasVolume := true
	for i := len(candles) - period; i < len(candles); i++ {
		if candles[i].Volume == 0 {
			hasVolume = false
			break
		}
	}

	if !hasVolume {
		return false, "NO_VOLUME_DATA", 0.0
	}

	// Calculate price change
	priceChange := candles[len(candles)-1].Close - candles[len(candles)-period].Close
	priceDirection := "UP"
	if priceChange < 0 {
		priceDirection = "DOWN"
	}

	// Calculate volume trend
	var volumeTrend int64
	for i := len(candles) - period + 1; i < len(candles); i++ {
		volumeDiff := candles[i].Volume - candles[i-1].Volume
		volumeTrend += volumeDiff
	}

	volumeDirection := "UP"
	if volumeTrend < 0 {
		volumeDirection = "DOWN"
	}

	// Detect divergence
	divergence := false
	divergenceType := ""
	divergenceStrength := 0.0

	if priceDirection == "UP" && volumeDirection == "DOWN" {
		divergence = true
		divergenceType = "BEARISH" // Bearish divergence: price up, volume down
		divergenceStrength = math.Abs(float64(volumeTrend)) / float64(candles[len(candles)-period].Volume) * 100
	} else if priceDirection == "DOWN" && volumeDirection == "UP" {
		divergence = true
		divergenceType = "BULLISH" // Bullish divergence: price down, volume up
		divergenceStrength = math.Abs(float64(volumeTrend)) / float64(candles[len(candles)-period].Volume) * 100
	}

	// Normalize divergence strength to 0-1 range
	if divergenceStrength > 100 {
		divergenceStrength = 1.0
	} else {
		divergenceStrength = divergenceStrength / 100
	}

	return divergence, divergenceType, divergenceStrength
}
