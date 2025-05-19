package utils

import (
	"fmt"
	"sync"

	"github.com/Alias1177/Predictor/internal/indicators"
	"github.com/Alias1177/Predictor/models"
)

// CalculateMarketFeatures рассчитывает основные характеристики рынка
func CalculateMarketFeatures(candles []models.Candle) ([]float64, error) {
	if len(candles) < 20 {
		return nil, fmt.Errorf("insufficient data: need at least 20 candles")
	}

	features := make([]float64, 4)
	var wg sync.WaitGroup
	results := make(chan struct {
		index int
		value float64
		err   error
	}, 4)

	// Волатильность
	wg.Add(1)
	go func() {
		defer wg.Done()
		atr := CalculateATR(candles, 14)
		results <- struct {
			index int
			value float64
			err   error
		}{0, atr, nil}
	}()

	// Тренд
	wg.Add(1)
	go func() {
		defer wg.Done()
		ema20 := indicators.CalculateEMA(candles, 20)
		ema50 := indicators.CalculateEMA(candles, 50)
		if ema50 == 0 {
			results <- struct {
				index int
				value float64
				err   error
			}{1, 0, fmt.Errorf("division by zero in trend calculation")}
			return
		}
		trend := (ema20 - ema50) / ema50
		results <- struct {
			index int
			value float64
			err   error
		}{1, trend, nil}
	}()

	// Моментум
	wg.Add(1)
	go func() {
		defer wg.Done()
		rsi := indicators.CalculateRSI(candles, 14)
		results <- struct {
			index int
			value float64
			err   error
		}{2, rsi, nil}
	}()

	// Объем
	wg.Add(1)
	go func() {
		defer wg.Done()
		volumeChange := CalculateVolumeChange(candles)
		results <- struct {
			index int
			value float64
			err   error
		}{3, volumeChange, nil}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		features[result.index] = result.value
	}

	return features, nil
}

// CalculateVolumeChange рассчитывает изменение объема
func CalculateVolumeChange(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0
	}

	recentVol := int64(0)
	oldVol := int64(0)

	for i := len(candles) - 10; i < len(candles); i++ {
		recentVol += candles[i].Volume
	}

	for i := len(candles) - 20; i < len(candles)-10; i++ {
		oldVol += candles[i].Volume
	}

	if oldVol == 0 {
		return 0
	}

	return float64(recentVol-oldVol) / float64(oldVol)
}
