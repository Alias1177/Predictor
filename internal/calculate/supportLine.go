package calculate

import (
	"github.com/Alias1177/Predictor/models"
	"math"
	"sort"
)

func identifySupportResistance(candles []models.Candle) ([]float64, []float64) {
	if len(candles) < 20 {
		return nil, nil
	}

	// Prepare price points map to track touch frequency
	pricePoints := make(map[float64]int)
	priceTolerance := 0.0002 // For EUR/USD, approximately 2 pips

	// Scan for swing highs and lows
	for i := 2; i < len(candles)-2; i++ {
		// Potential support (swing low)
		if candles[i].Low < candles[i-1].Low &&
			candles[i].Low < candles[i-2].Low &&
			candles[i].Low < candles[i+1].Low &&
			candles[i].Low < candles[i+2].Low {

			// Round to nearby level for clustering
			level := math.Round(candles[i].Low/priceTolerance) * priceTolerance
			pricePoints[level]++
		}

		// Potential resistance (swing high)
		if candles[i].High > candles[i-1].High &&
			candles[i].High > candles[i-2].High &&
			candles[i].High > candles[i+1].High &&
			candles[i].High > candles[i+2].High {

			// Round to nearby level for clustering
			level := math.Round(candles[i].High/priceTolerance) * priceTolerance
			pricePoints[level]++
		}
	}

	// Check for recent closes near these levels
	for i := len(candles) - 10; i < len(candles); i++ {
		for price := range pricePoints {
			// Check if close is near this level
			if math.Abs(candles[i].Close-price) < priceTolerance*2 {
				pricePoints[price]++
			}
		}
	}

	// Process and sort levels by strength
	type PriceLevel struct {
		Price    float64
		Strength int
	}

	var levels []PriceLevel
	for price, strength := range pricePoints {
		levels = append(levels, PriceLevel{Price: price, Strength: strength})
	}

	// Sort by strength (descending)
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Strength > levels[j].Strength
	})

	// Current price
	currentPrice := candles[len(candles)-1].Close

	// Separate into support and resistance
	var support, resistance []float64
	for _, level := range levels {
		if level.Price < currentPrice {
			support = append(support, level.Price)
		} else if level.Price > currentPrice {
			resistance = append(resistance, level.Price)
		}
	}

	// Sort support (descending - nearest first) and resistance (ascending - nearest first)
	sort.Slice(support, func(i, j int) bool {
		return support[i] > support[j]
	})

	sort.Slice(resistance, func(i, j int) bool {
		return resistance[i] < resistance[j]
	})

	// Limit to most significant levels
	maxLevels := 3
	if len(support) > maxLevels {
		support = support[:maxLevels]
	}
	if len(resistance) > maxLevels {
		resistance = resistance[:maxLevels]
	}

	return support, resistance
}
