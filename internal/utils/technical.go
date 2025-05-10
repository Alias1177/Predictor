package utils

import (
	"github.com/Alias1177/Predictor/models"
	"math"
)

// Copy the CalculateATR function from calculate/ATR.go
func CalculateATR(candles []models.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}

	var trueRanges []float64

	// Calculate True Range for each candle
	for i := 1; i < len(candles); i++ {
		// True Range is the greatest of:
		// 1. Current High - Current Low
		// 2. Abs(Current High - Previous Close)
		// 3. Abs(Current Low - Previous Close)
		highLow := candles[i].High - candles[i].Low
		highPrevClose := math.Abs(candles[i].High - candles[i-1].Close)
		lowPrevClose := math.Abs(candles[i].Low - candles[i-1].Close)

		trueRange := math.Max(highLow, math.Max(highPrevClose, lowPrevClose))
		trueRanges = append(trueRanges, trueRange)
	}

	// If we don't have enough data for the period, use what we have
	periodToUse := period
	if len(trueRanges) < period {
		periodToUse = len(trueRanges)
	}

	// Calculate average of true ranges
	var sum float64
	for i := len(trueRanges) - periodToUse; i < len(trueRanges); i++ {
		sum += trueRanges[i]
	}

	return sum / float64(periodToUse)
}

// Also add any other calculation functions that both packages need
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func CalculateADX(candles []models.Candle, period int) (float64, float64, float64) {
	if len(candles) < period*2 {
		return 0, 0, 0 // Not enough data
	}

	// Calculate +DM, -DM, and TR for each period
	var plusDM, minusDM, trueRange []float64

	for i := 1; i < len(candles); i++ {
		// Calculate +DM and -DM
		upMove := candles[i].High - candles[i-1].High
		downMove := candles[i-1].Low - candles[i].Low

		// +DM occurs when current high - previous high > previous low - current low
		// and is positive
		pDM := 0.0
		if upMove > downMove && upMove > 0 {
			pDM = upMove
		}
		plusDM = append(plusDM, pDM)

		// -DM occurs when previous low - current low > current high - previous high
		// and is positive
		mDM := 0.0
		if downMove > upMove && downMove > 0 {
			mDM = downMove
		}
		minusDM = append(minusDM, mDM)

		// Calculate True Range
		tr1 := candles[i].High - candles[i].Low
		tr2 := math.Abs(candles[i].High - candles[i-1].Close)
		tr3 := math.Abs(candles[i].Low - candles[i-1].Close)
		trueRange = append(trueRange, math.Max(tr1, math.Max(tr2, tr3)))
	}

	// Calculate initial ATR
	_ = CalculateAverage(trueRange[:period])

	// Calculate smoothed +DM14, -DM14, and TR14
	var smoothedPlusDM, smoothedMinusDM, smoothedTR float64

	// Initial values
	for i := 0; i < period; i++ {
		smoothedPlusDM += plusDM[i]
		smoothedMinusDM += minusDM[i]
		smoothedTR += trueRange[i]
	}

	// Calculate +DI and -DI
	plusDI := (smoothedPlusDM / smoothedTR) * 100
	minusDI := (smoothedMinusDM / smoothedTR) * 100

	// Calculate DX (Directional Index)
	dx := math.Abs(plusDI-minusDI) / (plusDI + minusDI) * 100

	// Calculate ADX
	adx := dx

	// Refine for remaining periods
	for i := period; i < len(trueRange); i++ {
		// Update smoothed values
		smoothedPlusDM = smoothedPlusDM - (smoothedPlusDM / float64(period)) + plusDM[i]
		smoothedMinusDM = smoothedMinusDM - (smoothedMinusDM / float64(period)) + minusDM[i]
		smoothedTR = smoothedTR - (smoothedTR / float64(period)) + trueRange[i]

		// Recalculate indicators
		newPlusDI := (smoothedPlusDM / smoothedTR) * 100
		newMinusDI := (smoothedMinusDM / smoothedTR) * 100

		newDX := math.Abs(newPlusDI-newMinusDI) / (newPlusDI + newMinusDI) * 100

		// ADX is smoothed DX
		adx = ((float64(period-1) * adx) + newDX) / float64(period)

		plusDI = newPlusDI
		minusDI = newMinusDI
	}

	return adx, plusDI, minusDI
}
func CalculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}
