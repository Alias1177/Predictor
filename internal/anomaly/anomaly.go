package anomaly

import (
	"fmt"
	"math"

	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
)

// detectMarketAnomalies identifies unusual market conditions
func DetectMarketAnomalies(candles []models.Candle) *models.AnomalyDetection {
	if len(candles) < 20 {
		return &models.AnomalyDetection{
			IsAnomaly:    false,
			AnomalyScore: 0,
		}
	}

	// Initialize result
	anomaly := &models.AnomalyDetection{
		IsAnomaly:        false,
		AnomalyScore:     0,
		RecommendedFlags: []string{},
	}

	// Get most recent candle
	current := candles[len(candles)-1]

	// Calculate recent volatility baseline
	atr10 := utils.CalculateATR(candles, 10)
	atr50 := utils.CalculateATR(candles, utils.MinInt(50, len(candles)-1))

	volatilityRatio := atr10 / atr50

	// 1. Check for price spikes
	prevCandle := candles[len(candles)-2]
	priceChange := math.Abs(current.Close - prevCandle.Close)
	normalizedPriceChange := priceChange / atr10

	if normalizedPriceChange > 3.0 {
		anomaly.IsAnomaly = true
		anomaly.AnomalyType = "PRICE_SPIKE"
		anomaly.AnomalyScore = math.Min(normalizedPriceChange/3.0, 1.0)
		anomaly.Details = fmt.Sprintf("Price moved %.1f times the normal range", normalizedPriceChange)
		anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "REDUCE_POSITION_SIZE", "USE_WIDER_STOPS")
	}

	// 2. Check for volume spikes (if volume data is available)
	if current.Volume > 0 {
		// Calculate average volume
		var totalVolume int64
		for i := len(candles) - 11; i < len(candles)-1; i++ {
			totalVolume += candles[i].Volume
		}
		avgVolume := float64(totalVolume) / 10.0

		volumeRatio := float64(current.Volume) / avgVolume

		if volumeRatio > 3.0 {
			// If we already detected a price anomaly, increase the score
			if anomaly.IsAnomaly {
				anomaly.AnomalyScore = math.Min(anomaly.AnomalyScore+0.2, 1.0)
				anomaly.AnomalyType += "_WITH_VOLUME_SPIKE"
			} else {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "VOLUME_SPIKE"
				anomaly.AnomalyScore = math.Min(volumeRatio/5.0, 1.0)
				anomaly.Details = fmt.Sprintf("Volume %.1f times the average", volumeRatio)
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "WAIT_FOR_CONFIRMATION")
			}
		}
	}

	// 3. Check for gaps
	if len(candles) > 2 {
		prevClose := prevCandle.Close
		gapSize := 0.0

		if current.Low > prevClose {
			// Gap up
			gapSize = current.Low - prevClose
		} else if current.High < prevClose {
			// Gap down
			gapSize = prevClose - current.High
		}

		normalizedGapSize := gapSize / atr10

		if normalizedGapSize > 1.0 {
			if anomaly.IsAnomaly {
				anomaly.AnomalyScore = math.Min(anomaly.AnomalyScore+0.15, 1.0)
				anomaly.AnomalyType += "_WITH_GAP"
			} else {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "GAP"
				anomaly.AnomalyScore = math.Min(normalizedGapSize/2.0, 1.0)
				anomaly.Details = fmt.Sprintf("Price gapped %.1f times the average range", normalizedGapSize)
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_VOLATILE_TRADING")
			}
		}
	}

	// 4. Check for pattern breaks
	if volatilityRatio > 2.5 {
		if anomaly.IsAnomaly {
			anomaly.AnomalyScore = math.Min(anomaly.AnomalyScore+0.1, 1.0)
		} else {
			anomaly.IsAnomaly = true
			anomaly.AnomalyType = "VOLATILITY_BREAKOUT"
			anomaly.AnomalyScore = math.Min(volatilityRatio/4.0, 1.0)
			anomaly.Details = fmt.Sprintf("Recent volatility %.1f times the baseline", volatilityRatio)
			anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_MOMENTUM", "ADJUST_TRADE_SIZE")
		}
	}

	// If any anomaly was detected, add common flags
	if anomaly.IsAnomaly {
		anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "USE_CAUTION", "MONITOR_CLOSELY")
	}

	return anomaly
}
