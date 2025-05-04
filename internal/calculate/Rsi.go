package calculate

import "chi/Predictor/models"

func calculateRSI(candles []models.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50.0 // Default value if not enough data
	}

	var gains, losses float64
	// Calculate initial averages
	for i := 1; i <= period; i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	// Calculate initial averages
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Use EMA (Exponential Moving Average) for the rest of the data
	for i := period + 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + 0) / float64(period)
		} else {
			avgGain = (avgGain*float64(period-1) + 0) / float64(period)
			avgLoss = (avgLoss*float64(period-1) - change) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}
