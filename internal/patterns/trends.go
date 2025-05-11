package patterns

import (
	"math"

	"github.com/Alias1177/Predictor/models"
)

func IdentifyTrends(candles []models.Candle, ema float64) []string {
	if len(candles) < 5 {
		return []string{"Insufficient data for trend analysis"}
	}

	var trends []string

	// Check short-term trend (last 5 candles)
	shortTermUp := true
	shortTermDown := true

	for i := len(candles) - 5; i < len(candles)-1; i++ {
		if candles[i].Close > candles[i+1].Close {
			shortTermUp = false
		}
		if candles[i].Close < candles[i+1].Close {
			shortTermDown = false
		}
	}

	// Check position relative to EMA
	lastClose := candles[len(candles)-1].Close
	if lastClose > ema {
		trends = append(trends, "Price above EMA (bullish)")
	} else if lastClose < ema {
		trends = append(trends, "Price below EMA (bearish)")
	}

	// Add short term trend identification
	if shortTermUp {
		trends = append(trends, "Strong uptrend in last 5 candles")
	} else if shortTermDown {
		trends = append(trends, "Strong downtrend in last 5 candles")
	}

	// Check for potential reversal patterns
	lastCandle := candles[len(candles)-1]
	prevCandle := candles[len(candles)-2]

	// Bullish engulfing
	if lastCandle.Close > lastCandle.Open &&
		prevCandle.Close < prevCandle.Open &&
		lastCandle.Open < prevCandle.Close &&
		lastCandle.Close > prevCandle.Open {
		trends = append(trends, "Potential bullish engulfing pattern")
	}

	// Bearish engulfing
	if lastCandle.Close < lastCandle.Open &&
		prevCandle.Close > prevCandle.Open &&
		lastCandle.Open > prevCandle.Close &&
		lastCandle.Close < prevCandle.Open {
		trends = append(trends, "Potential bearish engulfing pattern")
	}

	// Check momentum
	if len(candles) >= 10 {
		momentum := lastCandle.Close - candles[len(candles)-10].Close
		if momentum > 0 && momentum/candles[len(candles)-10].Close > 0.01 {
			trends = append(trends, "Strong positive momentum")
		} else if momentum < 0 && -momentum/candles[len(candles)-10].Close > 0.01 {
			trends = append(trends, "Strong negative momentum")
		}
	}

	return trends
}

func DetectTrendAlignment(mtfData map[string][]models.Candle, config *models.Config) (string, float64) {
	// Calculate EMAs for each timeframe
	trendScores := make(map[string]float64)

	for timeframe, candles := range mtfData {
		if len(candles) < 3 {
			continue
		}

		// Get last 3 closes
		last := candles[len(candles)-1].Close
		prev := candles[len(candles)-2].Close
		prevPrev := candles[len(candles)-3].Close

		// Calculate fast and slow EMAs
		fastEMA := CalculateEMA(candles, 8)
		slowEMA := CalculateEMA(candles, 21)

		// Determine trend direction and strength
		trendDirection := 0.0
		if last > prev && prev > prevPrev {
			trendDirection = 1.0 // Uptrend
		} else if last < prev && prev < prevPrev {
			trendDirection = -1.0 // Downtrend
		}

		// EMA alignment factor
		emaAlignment := 0.0
		if fastEMA > slowEMA {
			emaAlignment = 1.0
		} else if fastEMA < slowEMA {
			emaAlignment = -1.0
		}

		// Price position relative to EMAs
		pricePosition := 0.0
		if last > fastEMA && last > slowEMA {
			pricePosition = 1.0
		} else if last < fastEMA && last < slowEMA {
			pricePosition = -1.0
		}

		// Weight different timeframes (higher timeframes have more weight)
		weight := 1.0
		if timeframe == "5min" {
			weight = 2.0
		} else if timeframe == "15min" {
			weight = 3.0
		}

		// Combined score for this timeframe
		trendScores[timeframe] = (trendDirection + emaAlignment + pricePosition) * weight
	}

	// Calculate overall alignment score
	var totalScore float64
	for _, score := range trendScores {
		totalScore += score
	}

	// Normalize score to range [-1, 1]
	maxPossibleScore := 6.0 // 3 metrics * max weight of 2
	if len(trendScores) > 0 {
		alignmentScore := totalScore / maxPossibleScore

		// Determine alignment direction
		alignmentDirection := "NEUTRAL"
		alignmentStrength := math.Abs(alignmentScore)

		if alignmentScore > 0.3 {
			alignmentDirection = "BULLISH"
		} else if alignmentScore < -0.3 {
			alignmentDirection = "BEARISH"
		}

		return alignmentDirection, alignmentStrength
	}

	return "NEUTRAL", 0.0
}
