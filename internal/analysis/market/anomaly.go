package market

import (
	"chi/Predictor/internal/analysis/technical"
	"chi/Predictor/internal/model"
	"fmt"
	"math"
)

// DetectMarketAnomalies identifies unusual market conditions
func DetectMarketAnomalies(candles []model.Candle) *model.AnomalyDetection {
	if len(candles) < 20 {
		return &model.AnomalyDetection{
			IsAnomaly:    false,
			AnomalyScore: 0,
		}
	}

	// Initialize result
	anomaly := &model.AnomalyDetection{
		IsAnomaly:        false,
		AnomalyScore:     0,
		RecommendedFlags: []string{},
	}

	// Get most recent candle
	current := candles[len(candles)-1]

	// Calculate recent volatility baseline
	atr10 := technical.CalculateATR(candles, 10)
	atr50 := technical.CalculateATR(candles, min(50, len(candles)-1))

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

	// 5. Check for extreme RSI values
	rsi := technical.CalculateRSI(candles, 14)
	if rsi < 10 || rsi > 90 {
		if anomaly.IsAnomaly {
			anomaly.AnomalyScore = math.Min(anomaly.AnomalyScore+0.1, 1.0)
		} else {
			anomaly.IsAnomaly = true
			anomaly.AnomalyType = "EXTREME_OVERBOUGHT_OVERSOLD"
			if rsi < 10 {
				anomaly.AnomalyScore = (10 - rsi) / 10
				anomaly.Details = fmt.Sprintf("Extremely oversold RSI: %.1f", rsi)
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_REVERSAL", "LIMIT_SHORT_EXPOSURE")
			} else {
				anomaly.AnomalyScore = (rsi - 90) / 10
				anomaly.Details = fmt.Sprintf("Extremely overbought RSI: %.1f", rsi)
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_REVERSAL", "LIMIT_LONG_EXPOSURE")
			}
		}
	}

	// 6. Check for extreme price movements over multiple periods
	if len(candles) >= 10 {
		priceChange5 := (current.Close - candles[len(candles)-6].Close) / candles[len(candles)-6].Close
		if math.Abs(priceChange5) > 0.05 { // 5% change in 5 candles
			if anomaly.IsAnomaly {
				anomaly.AnomalyScore = math.Min(anomaly.AnomalyScore+0.15, 1.0)
			} else {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "RAPID_PRICE_MOVE"
				anomaly.AnomalyScore = math.Min(math.Abs(priceChange5)/0.1, 1.0) // 10% move = max score
				if priceChange5 > 0 {
					anomaly.Details = fmt.Sprintf("Rapid %.1f%% price increase", priceChange5*100)
				} else {
					anomaly.Details = fmt.Sprintf("Rapid %.1f%% price decrease", -priceChange5*100)
				}
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_PULLBACK")
			}
		}
	}

	// If any anomaly was detected, add common flags
	if anomaly.IsAnomaly {
		anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "USE_CAUTION", "MONITOR_CLOSELY")
	}

	return anomaly
}

// DetectPatternBreakdown identifies when established patterns are breaking down
func DetectPatternBreakdown(candles []model.Candle, regime *model.MarketRegime) *model.AnomalyDetection {
	if len(candles) < 30 || regime == nil {
		return &model.AnomalyDetection{
			IsAnomaly:    false,
			AnomalyScore: 0,
		}
	}

	// Initialize result
	anomaly := &model.AnomalyDetection{
		IsAnomaly:        false,
		AnomalyScore:     0,
		RecommendedFlags: []string{},
	}

	// Check for breakdown of ranging market
	if regime.Type == "RANGING" && regime.Strength > 0.7 {
		support, resistance := technical.IdentifySupportResistance(candles)

		// Check if price is breaking out of the range
		currentPrice := candles[len(candles)-1].Close

		// Check if we have valid support/resistance levels
		if len(support) > 0 && len(resistance) > 0 {
			lowestSupport := support[len(support)-1]
			highestResistance := resistance[0]

			// Check for breakouts
			if currentPrice < lowestSupport*0.99 { // 1% below lowest support
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "RANGE_BREAKDOWN"
				anomaly.AnomalyScore = 0.7
				anomaly.Details = "Price breaking below established range support"
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "RANGE_TO_TREND_TRANSITION", "EXPECT_FOLLOWTHROUGH")
			} else if currentPrice > highestResistance*1.01 { // 1% above highest resistance
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "RANGE_BREAKOUT"
				anomaly.AnomalyScore = 0.7
				anomaly.Details = "Price breaking above established range resistance"
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "RANGE_TO_TREND_TRANSITION", "EXPECT_FOLLOWTHROUGH")
			}
		}
	}

	// Check for trend exhaustion
	if regime.Type == "TRENDING" && regime.Strength > 0.7 {
		// Calculate momentum and check for divergence with price
		adx, plusDI, minusDI := technical.CalculateADX(candles, 14)
		rsi := technical.CalculateRSI(candles, 14)

		// Check the last 5 candles for divergence
		var recentPrices []float64
		for i := len(candles) - 5; i < len(candles); i++ {
			recentPrices = append(recentPrices, candles[i].Close)
		}

		// Check for trend exhaustion signs
		if regime.Direction == "BULLISH" {
			// In an uptrend, check for bearish divergence or ADX downturn
			if adx < technical.CalculateADXValue(candles[:len(candles)-1], 14) && adx > 25 {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "TREND_EXHAUSTION"
				anomaly.AnomalyScore = 0.6
				anomaly.Details = "Strong uptrend showing signs of exhaustion (ADX turning down)"
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "REDUCE_TREND_FOLLOWING_EXPOSURE", "WATCH_FOR_REVERSAL")
			}

			// Check for bearish RSI divergence
			if isNewHigh(recentPrices) && rsi < calculatePreviousRSI(candles, 5) && rsi > 70 {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "BEARISH_DIVERGENCE"
				anomaly.AnomalyScore = 0.8
				anomaly.Details = "Price making new highs with weakening momentum (RSI divergence)"
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_REVERSAL", "PROTECT_PROFITS")
			}
		} else if regime.Direction == "BEARISH" {
			// In a downtrend, check for bullish divergence or ADX downturn
			if adx < technical.CalculateADXValue(candles[:len(candles)-1], 14) && adx > 25 {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "TREND_EXHAUSTION"
				anomaly.AnomalyScore = 0.6
				anomaly.Details = "Strong downtrend showing signs of exhaustion (ADX turning down)"
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "REDUCE_TREND_FOLLOWING_EXPOSURE", "WATCH_FOR_REVERSAL")
			}

			// Check for bullish RSI divergence
			if isNewLow(recentPrices) && rsi > calculatePreviousRSI(candles, 5) && rsi < 30 {
				anomaly.IsAnomaly = true
				anomaly.AnomalyType = "BULLISH_DIVERGENCE"
				anomaly.AnomalyScore = 0.8
				anomaly.Details = "Price making new lows with weakening momentum (RSI divergence)"
				anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "EXPECT_REVERSAL", "PROTECT_PROFITS")
			}
		}
	}

	// If any anomaly was detected, add common flags
	if anomaly.IsAnomaly {
		anomaly.RecommendedFlags = append(anomaly.RecommendedFlags, "ADJUST_RISK_MANAGEMENT", "MONITOR_CLOSELY")
	}

	return anomaly
}

// Helper functions for pattern breakdown detection
func isNewHigh(prices []float64) bool {
	if len(prices) < 5 {
		return false
	}
	return prices[len(prices)-1] > prices[len(prices)-2] &&
		prices[len(prices)-1] > prices[len(prices)-3] &&
		prices[len(prices)-1] > prices[len(prices)-4] &&
		prices[len(prices)-1] > prices[len(prices)-5]
}

func isNewLow(prices []float64) bool {
	if len(prices) < 5 {
		return false
	}
	return prices[len(prices)-1] < prices[len(prices)-2] &&
		prices[len(prices)-1] < prices[len(prices)-3] &&
		prices[len(prices)-1] < prices[len(prices)-4] &&
		prices[len(prices)-1] < prices[len(prices)-5]
}

func calculatePreviousRSI(candles []model.Candle, offset int) float64 {
	// Calculate RSI from previous candles
	if len(candles) < offset+14 {
		return 50.0
	}

	previousCandles := candles[:len(candles)-offset]
	return technical.CalculateRSI(previousCandles, 14)
}

// Placeholder for ADX calculation from technical package
func technical_CalculateADXValue(candles []model.Candle, period int) float64 {
	adx, _, _ := technical.CalculateADX(candles, period)
	return adx
}
