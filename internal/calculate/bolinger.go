package calculate

import (
	"github.com/Alias1177/Predictor/models"
	"math"
)

// calculateBollingerBands calculates Bollinger Bands
func calculateBollingerBands(candles []models.Candle, period int, stdDev float64) (float64, float64, float64) {
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
