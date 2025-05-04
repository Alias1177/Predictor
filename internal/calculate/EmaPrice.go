package calculate

func calculateEMAFromPrices(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1] // Return last price if not enough data
	}

	// Calculate simple moving average for the initial value
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	sma := sum / float64(period)

	// Multiplier for weighting the EMA
	multiplier := 2.0 / float64(period+1)

	// Start with SMA and then calculate EMA
	ema := sma
	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}
