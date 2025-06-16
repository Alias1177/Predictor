package utils

import (
	"fmt"
	"math"
	"sync"

	"github.com/Alias1177/Predictor/models"
)

// CalculateMarketFeatures вычисляет основные рыночные признаки
func CalculateMarketFeatures(candles []models.Candle) ([]float64, error) {
	if len(candles) < 20 {
		return nil, fmt.Errorf("insufficient data: need at least 20 candles, got %d", len(candles))
	}

	features := make([]float64, 4)
	var wg sync.WaitGroup
	var errChan = make(chan error, 4)

	// Волатильность (ATR)
	wg.Add(1)
	go func() {
		defer wg.Done()
		atr := calculateATR(candles, 14)
		if atr == 0 {
			errChan <- fmt.Errorf("failed to calculate ATR")
			return
		}
		features[0] = atr / candles[len(candles)-1].Close
	}()

	// Тренд (EMA)
	wg.Add(1)
	go func() {
		defer wg.Done()
		ema := calculateEMA(candles, 20)
		if len(ema) == 0 {
			errChan <- fmt.Errorf("failed to calculate EMA")
			return
		}
		features[1] = (ema[len(ema)-1] - ema[len(ema)-2]) / ema[len(ema)-2]
	}()

	// Моментум (RSI)
	wg.Add(1)
	go func() {
		defer wg.Done()
		rsi := calculateRSI(candles, 14)
		if len(rsi) == 0 {
			errChan <- fmt.Errorf("failed to calculate RSI")
			return
		}
		features[2] = rsi[len(rsi)-1]
	}()

	// Объем
	wg.Add(1)
	go func() {
		defer wg.Done()
		volumeChange := calculateVolumeChange(candles, 20)
		if volumeChange == 0 {
			errChan <- fmt.Errorf("failed to calculate volume change")
			return
		}
		features[3] = volumeChange
	}()

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return features, nil
}

// calculateATR вычисляет Average True Range
func calculateATR(candles []models.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}

	tr := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		high := candles[i].High
		low := candles[i].Low
		prevClose := candles[i-1].Close

		tr[i-1] = math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
	}

	atr := 0.0
	for i := 0; i < period; i++ {
		atr += tr[i]
	}

	if period == 0 {
		return 0
	}

	atr /= float64(period)

	return atr
}

// calculateEMA вычисляет Exponential Moving Average
func calculateEMA(candles []models.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	ema := make([]float64, len(candles))
	multiplier := 2.0 / float64(period+1)

	// Первое значение EMA - это SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	ema[period-1] = sum / float64(period)

	// Остальные значения EMA
	for i := period; i < len(candles); i++ {
		ema[i] = (candles[i].Close-ema[i-1])*multiplier + ema[i-1]
	}

	return ema[period-1:]
}

// calculateRSI вычисляет Relative Strength Index
func calculateRSI(candles []models.Candle, period int) []float64 {
	if len(candles) < period+1 {
		return nil
	}

	gains := make([]float64, len(candles)-1)
	losses := make([]float64, len(candles)-1)

	for i := 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		if change >= 0 {
			gains[i-1] = change
		} else {
			losses[i-1] = -change
		}
	}

	rsi := make([]float64, len(candles)-period)
	avgGain := 0.0
	avgLoss := 0.0

	// Первое значение RSI
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		rsi[0] = 100
	} else {
		rs := avgGain / avgLoss
		rsi[0] = 100 - (100 / (1 + rs))
	}

	// Остальные значения RSI
	for i := 1; i < len(rsi); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i+period-1]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i+period-1]) / float64(period)

		if avgLoss == 0 {
			rsi[i] = 100
		} else {
			rs := avgGain / avgLoss
			rsi[i] = 100 - (100 / (1 + rs))
		}
	}

	return rsi
}

// calculateVolumeChange вычисляет изменение объема
func calculateVolumeChange(candles []models.Candle, period int) float64 {
	if len(candles) < period*2 {
		return 0
	}

	recentVolume := int64(0)
	oldVolume := int64(0)

	for i := len(candles) - period; i < len(candles); i++ {
		recentVolume += candles[i].Volume
	}

	for i := len(candles) - period*2; i < len(candles)-period; i++ {
		oldVolume += candles[i].Volume
	}

	if oldVolume == 0 {
		return 0
	}

	return float64(recentVolume-oldVolume) / float64(oldVolume)
}
