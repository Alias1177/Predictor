package calculate

import (
	"context"
	"fmt"
	"sync"

	"github.com/Alias1177/Predictor/config"
	"github.com/Alias1177/Predictor/models"
)

// getMultiTimeframeData fetches candle data for multiple timeframes
func GetMultiTimeframeData(ctx context.Context, apiKey string, symbol string) (map[string][]models.Candle, error) {
	// Updated to use supported intervals from Twelve Data API
	timeframes := map[string]string{
		"1min":  "1min",
		"5min":  "5min",
		"15min": "15min",
	}

	result := make(map[string][]models.Candle)

	// Use a waitgroup to fetch data in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for name, interval := range timeframes {
		wg.Add(1)

		go func(name, interval string) {
			defer wg.Done()

			// Create a new config for different interval
			// Use the client's API to get or create a configuration instead of accessing private fields
			tempConfig := models.Config{
				Symbol:       symbol,
				Interval:     interval,
				CandleCount:  30,
				TwelveAPIKey: apiKey,
			}
			if interval == "15min" {
				tempConfig.CandleCount = 20
			}

			tempClient := config.NewClient(&tempConfig)

			candles, err := tempClient.GetCandles(ctx)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to fetch %s candles: %w", name, err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			result[name] = candles
			mu.Unlock()
		}(name, interval)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}
