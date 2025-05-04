package technical

import "chi/Predictor/internal/model"

// CalculateRSI calculates the Relative Strength Index
func CalculateRSI(candles []model.Candle, period int) float64 {
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

// CalculateStochastic calculates the Stochastic Oscillator
func CalculateStochastic(candles []model.Candle, kPeriod, dPeriod int) (float64, float64) {
	if len(candles) < kPeriod {
		return 50.0, 50.0 // Default values if not enough data
	}

	// Calculate %K
	var highest, lowest float64
	latest := candles[len(candles)-1]

	// Find highest high and lowest low in the lookback period
	for i := len(candles) - kPeriod; i < len(candles); i++ {
		if i == len(candles)-kPeriod || candles[i].High > highest {
			highest = candles[i].High
		}
		if i == len(candles)-kPeriod || candles[i].Low < lowest {
			lowest = candles[i].Low
		}
	}

	// Calculate %K
	var k float64
	if highest-lowest > 0 {
		k = ((latest.Close - lowest) / (highest - lowest)) * 100
	} else {
		k = 50.0 // If no range, default to middle
	}

	// Calculate %D (simple moving average of %K)
	var kSum float64
	count := minInt(dPeriod, len(candles))

	for i := 0; i < count; i++ {
		// For each point, calculate its own %K
		startIdx := len(candles) - count + i - kPeriod + 1
		if startIdx < 0 {
			// Not enough data, use current K
			kSum += k
			continue
		}

		// Find highest and lowest for this K calculation
		var pointHighest, pointLowest float64
		pointCandle := candles[len(candles)-count+i]

		for j := 0; j < kPeriod && startIdx+j < len(candles); j++ {
			c := candles[startIdx+j]
			if j == 0 || c.High > pointHighest {
				pointHighest = c.High
			}
			if j == 0 || c.Low < pointLowest {
				pointLowest = c.Low
			}
		}

		// Add this point's %K to the sum
		if pointHighest-pointLowest > 0 {
			kSum += ((pointCandle.Close - pointLowest) / (pointHighest - pointLowest)) * 100
		} else {
			kSum += 50.0
		}
	}

	d := kSum / float64(count)

	return k, d
}

// Helper function
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
