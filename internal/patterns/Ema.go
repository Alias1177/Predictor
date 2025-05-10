package patterns

import "github.com/Alias1177/Predictor/models"

func CalculateEMA(candles []models.Candle, period int) float64 {
	if len(candles) < period {
		return candles[len(candles)-1].Close // Return last close if not enough data
	}

	// Calculate simple moving average for the initial value
	var sum float64
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	sma := sum / float64(period)

	// Multiplier for weighting the EMA
	multiplier := 2.0 / float64(period+1)

	// Start with SMA and then calculate EMA
	ema := sma
	for i := period; i < len(candles); i++ {
		ema = (candles[i].Close-ema)*multiplier + ema
	}

	return ema
}
