package calculate

import "github.com/Alias1177/Predictor/models"

func calculateOBV(candles []models.Candle) float64 {
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
