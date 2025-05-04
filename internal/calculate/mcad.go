package calculate

import "chi/Predictor/models"

func calculateMACD(candles []models.Candle, fastPeriod, slowPeriod, signalPeriod int) (float64, float64, float64) {
	// Extract close prices
	closes := make([]float64, len(candles))
	for i, candle := range candles {
		closes[i] = candle.Close
	}

	// Cannot calculate MACD with insufficient data
	if len(closes) < slowPeriod+signalPeriod {
		return 0, 0, 0
	}

	// Calculate EMAs once
	fastEMA := calculateEMAFromPrices(closes, fastPeriod)
	slowEMA := calculateEMAFromPrices(closes, slowPeriod)

	// Calculate MACD line
	macdLine := fastEMA - slowEMA

	// Calculate signal line (EMA of MACD line)
	// First, calculate MACD history
	macdHistory := make([]float64, 0, len(closes)-slowPeriod+1)

	// First calculate all the MACD values over time
	for i := slowPeriod - 1; i < len(closes); i++ {
		// Use a sliding window approach
		windowFast := closes[:i+1]
		windowSlow := closes[:i+1]
		fastEMA := calculateEMAFromPrices(windowFast, fastPeriod)
		slowEMA := calculateEMAFromPrices(windowSlow, slowPeriod)
		macdHistory = append(macdHistory, fastEMA-slowEMA)
	}

	// Now calculate the signal line (EMA of MACD)
	signalLine := 0.0
	if len(macdHistory) >= signalPeriod {
		signalLine = calculateEMAFromPrices(macdHistory, signalPeriod)
	}

	// MACD histogram
	histogram := macdLine - signalLine

	return macdLine, signalLine, histogram
}
