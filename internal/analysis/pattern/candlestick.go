package pattern

import (
	"chi/Predictor/internal/model"
	"math"
)

// IdentifyPriceActionPatterns identifies specific candle patterns
func IdentifyPriceActionPatterns(candles []model.Candle) []string {
	if len(candles) < 5 {
		return nil
	}

	var patterns []string

	// Get recent candles
	c1 := candles[len(candles)-5] // Oldest
	c2 := candles[len(candles)-4]
	c3 := candles[len(candles)-3]
	c4 := candles[len(candles)-2]
	c5 := candles[len(candles)-1] // Most recent

	// Body sizes
	bodySize1 := math.Abs(c1.Close - c1.Open)
	bodySize2 := math.Abs(c2.Close - c2.Open)
	bodySize3 := math.Abs(c3.Close - c3.Open)
	bodySize4 := math.Abs(c4.Close - c4.Open)
	bodySize5 := math.Abs(c5.Close - c5.Open)

	// Average body size
	avgBodySize := (bodySize1 + bodySize2 + bodySize3 + bodySize4 + bodySize5) / 5

	// Candle directions
	bullish3 := c3.Close > c3.Open
	bullish4 := c4.Close > c4.Open
	bullish5 := c5.Close > c5.Open

	// Upper and lower wicks
	upperWick5 := c5.High - math.Max(c5.Open, c5.Close)
	lowerWick5 := math.Min(c5.Open, c5.Close) - c5.Low

	// Check for engulfing patterns
	if bullish5 && !bullish4 &&
		c5.Open < c4.Close &&
		c5.Close > c4.Open &&
		bodySize5 > bodySize4*1.2 {
		patterns = append(patterns, "BULLISH_ENGULFING")
	}

	if !bullish5 && bullish4 &&
		c5.Open > c4.Close &&
		c5.Close < c4.Open &&
		bodySize5 > bodySize4*1.2 {
		patterns = append(patterns, "BEARISH_ENGULFING")
	}

	// Check for pin bars / hammers
	if lowerWick5 > bodySize5*2 && upperWick5 < bodySize5*0.5 {
		patterns = append(patterns, "HAMMER")
	}

	if upperWick5 > bodySize5*2 && lowerWick5 < bodySize5*0.5 {
		patterns = append(patterns, "SHOOTING_STAR")
	}

	// Check for three candle patterns
	if bullish3 && bullish4 && bullish5 {
		patterns = append(patterns, "THREE_WHITE_SOLDIERS")
	}

	if !bullish3 && !bullish4 && !bullish5 {
		patterns = append(patterns, "THREE_BLACK_CROWS")
	}

	// Check for doji
	if bodySize5 < avgBodySize*0.3 &&
		(upperWick5 > bodySize5 || lowerWick5 > bodySize5) {
		patterns = append(patterns, "DOJI")
	}

	// Check for momentum candles
	if bullish5 && bodySize5 > avgBodySize*1.5 &&
		lowerWick5 < bodySize5*0.2 && upperWick5 < bodySize5*0.2 {
		patterns = append(patterns, "STRONG_BULLISH_MOMENTUM")
	}

	if !bullish5 && bodySize5 > avgBodySize*1.5 &&
		lowerWick5 < bodySize5*0.2 && upperWick5 < bodySize5*0.2 {
		patterns = append(patterns, "STRONG_BEARISH_MOMENTUM")
	}

	// Evening Star Pattern (Bearish Reversal)
	if len(candles) >= 7 &&
		bullish3 && // First candle bullish
		bodySize3 > avgBodySize && // First candle has large body
		math.Abs(c4.Close-c4.Open) < avgBodySize*0.3 && // Middle candle has small body
		c4.Open > c3.Close && // Gap up between first and middle
		!bullish5 && // Third candle bearish
		bodySize5 > avgBodySize && // Third candle has large body
		c5.Close < (c3.Open+(c3.Close-c3.Open)/2) { // Third candle closes below midpoint of first
		patterns = append(patterns, "EVENING_STAR")
	}

	// Morning Star Pattern (Bullish Reversal)
	if len(candles) >= 7 &&
		!bullish3 && // First candle bearish
		bodySize3 > avgBodySize && // First candle has large body
		math.Abs(c4.Close-c4.Open) < avgBodySize*0.3 && // Middle candle has small body
		c4.Open < c3.Close && // Gap down between first and middle
		bullish5 && // Third candle bullish
		bodySize5 > avgBodySize && // Third candle has large body
		c5.Close > (c3.Open+(c3.Close-c3.Open)/2) { // Third candle closes above midpoint of first
		patterns = append(patterns, "MORNING_STAR")
	}

	// Double Top/Bottom patterns
	doubleTopBottomPatterns := identifyDoubleTopBottomPatterns(candles, avgBodySize)
	if len(doubleTopBottomPatterns) > 0 {
		patterns = append(patterns, doubleTopBottomPatterns...)
	}

	return patterns
}

// identifyDoubleTopBottomPatterns detects double top and double bottom patterns
func identifyDoubleTopBottomPatterns(candles []model.Candle, avgBodySize float64) []string {
	var patterns []string

	if len(candles) < 10 {
		return patterns
	}

	// Find Double Top pattern
	// Find two peaks with a valley in between
	var peaks []int
	for i := 2; i < len(candles)-2; i++ {
		if candles[i].High > candles[i-1].High &&
			candles[i].High > candles[i-2].High &&
			candles[i].High > candles[i+1].High &&
			candles[i].High > candles[i+2].High {
			peaks = append(peaks, i)
		}
	}

	if len(peaks) >= 2 {
		// Check if the last two peaks are of similar height
		last := peaks[len(peaks)-1]
		prev := peaks[len(peaks)-2]

		if math.Abs(candles[last].High-candles[prev].High) < avgBodySize*0.5 &&
			last-prev >= 3 { // Ensure some distance between peaks
			// Find the valley in between
			var lowestVal float64 = candles[prev].High
			lowestIdx := prev

			for i := prev + 1; i < last; i++ {
				if candles[i].Low < lowestVal {
					lowestVal = candles[i].Low
					lowestIdx = i
				}
			}

			// Check if the current price is below the valley
			if candles[len(candles)-1].Close < lowestVal {
				patterns = append(patterns, "DOUBLE_TOP")
			}

			// Use lowestIdx variable to avoid unused variable warning
			_ = lowestIdx
		}
	}

	// Find Double Bottom pattern
	// Find two troughs with a peak in between
	var troughs []int
	for i := 2; i < len(candles)-2; i++ {
		if candles[i].Low < candles[i-1].Low &&
			candles[i].Low < candles[i-2].Low &&
			candles[i].Low < candles[i+1].Low &&
			candles[i].Low < candles[i+2].Low {
			troughs = append(troughs, i)
		}
	}

	if len(troughs) >= 2 {
		// Check if the last two troughs are of similar height
		last := troughs[len(troughs)-1]
		prev := troughs[len(troughs)-2]

		if math.Abs(candles[last].Low-candles[prev].Low) < avgBodySize*0.5 &&
			last-prev >= 3 { // Ensure some distance between troughs
			// Find the peak in between
			var highestVal float64 = candles[prev].Low
			highestIdx := prev

			for i := prev + 1; i < last; i++ {
				if candles[i].High > highestVal {
					highestVal = candles[i].High
					highestIdx = i
				}
			}

			// Check if the current price is above the peak
			if candles[len(candles)-1].Close > highestVal {
				patterns = append(patterns, "DOUBLE_BOTTOM")
			}

			// Use highestIdx variable to avoid unused variable warning
			_ = highestIdx
		}
	}

	return patterns
}

// CalculatePatternStrength evaluates the strength of a given pattern
func CalculatePatternStrength(pattern string, candles []model.Candle) float64 {
	if len(candles) < 5 {
		return 0.0
	}

	// Recent candles
	c3 := candles[len(candles)-3]
	c4 := candles[len(candles)-2]
	c5 := candles[len(candles)-1] // Most recent

	// Calculate pattern strength based on type
	switch pattern {
	case "BULLISH_ENGULFING":
		// Strength based on how much current candle engulfs previous
		bodySize4 := math.Abs(c4.Close - c4.Open)
		bodySize5 := math.Abs(c5.Close - c5.Open)
		return (bodySize5 / bodySize4) - 1.0 // Strength above 0.2 (20% larger)

	case "BEARISH_ENGULFING":
		// Strength based on how much current candle engulfs previous
		bodySize4 := math.Abs(c4.Close - c4.Open)
		bodySize5 := math.Abs(c5.Close - c5.Open)
		return (bodySize5 / bodySize4) - 1.0 // Strength above 0.2 (20% larger)

	case "HAMMER", "SHOOTING_STAR":
		// Strength based on wick to body ratio
		bodySize := math.Abs(c5.Close - c5.Open)
		upperWick := c5.High - math.Max(c5.Open, c5.Close)
		lowerWick := math.Min(c5.Open, c5.Close) - c5.Low
		if pattern == "HAMMER" {
			if bodySize == 0 {
				return 0.0
			}
			return lowerWick / bodySize
		} else {
			if bodySize == 0 {
				return 0.0
			}
			return upperWick / bodySize
		}

	case "DOJI":
		// Strength based on how small the body is relative to the range
		bodySize := math.Abs(c5.Close - c5.Open)
		totalRange := c5.High - c5.Low
		if totalRange == 0 {
			return 0.0
		}
		return 1.0 - (bodySize / totalRange)

	case "THREE_WHITE_SOLDIERS", "THREE_BLACK_CROWS":
		// Strength based on consistency of candle sizes and minimal wicks
		bodySize3 := math.Abs(c3.Close - c3.Open)
		bodySize4 := math.Abs(c4.Close - c4.Open)
		bodySize5 := math.Abs(c5.Close - c5.Open)
		avgSize := (bodySize3 + bodySize4 + bodySize5) / 3
		sizeDiff := math.Abs(bodySize3-avgSize) + math.Abs(bodySize4-avgSize) + math.Abs(bodySize5-avgSize)
		consistency := 1.0 - (sizeDiff / (avgSize * 3))
		return consistency

	case "MORNING_STAR", "EVENING_STAR":
		// Strength based on middle candle size and reversal magnitude
		bodySize3 := math.Abs(c3.Close - c3.Open)
		bodySize4 := math.Abs(c4.Close - c4.Open)
		bodySize5 := math.Abs(c5.Close - c5.Open)
		middleRatio := 1.0 - (bodySize4 / ((bodySize3 + bodySize5) / 2))
		return middleRatio

	default:
		return 0.5 // Default moderate strength
	}
}
