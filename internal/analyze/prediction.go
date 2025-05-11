package analyze

import (
	"fmt"
	"github.com/Alias1177/Predictor/internal/calculate"
	"math"

	"github.com/Alias1177/Predictor/internal/patterns"
	"github.com/Alias1177/Predictor/models"
)

func EnhancedPrediction(
	candles []models.Candle,
	indicators *models.TechnicalIndicators,
	mtfData map[string][]models.Candle,
	regime *models.MarketRegime,
	anomaly *models.AnomalyDetection,
	config *models.Config) (string, string, float64, []string, *models.TradingSuggestion) {

	// 1. Multi-timeframe trend alignment
	trendDirection, trendStrength := patterns.DetectTrendAlignment(mtfData, config)

	// 2. Price action patterns
	patterns2 := patterns.IdentifyPriceActionPatterns(candles)

	// 3. Volatility conditions
	volatilityRegime, expectedMove := assessVolatilityConditions(candles)

	// 4. Order flow analysis
	flowDirection, _ := analyzeOrderFlow(candles)

	divergences := patterns.DetectDivergences(candles, indicators)

	// 5. Key levels proximity
	currentPrice := candles[len(candles)-1].Close
	nearestSupport := 0.0
	nearestResistance := 99999.0

	for _, level := range indicators.Support {
		if level < currentPrice && level > nearestSupport {
			nearestSupport = level
		}
	}

	for _, level := range indicators.Resistance {
		if level > currentPrice && level < nearestResistance {
			nearestResistance = level
		}
	}

	distanceToSupport := currentPrice - nearestSupport
	distanceToResistance := nearestResistance - currentPrice

	// 6. Scoring model for direction prediction

	// Base scores
	bullishScore := 0.0
	bearishScore := 0.0

	// Market regime factor (higher weight)
	if regime.Type == "TRENDING" {
		if regime.Direction == "BULLISH" {
			bullishScore += 2.0 * regime.Strength
		} else if regime.Direction == "BEARISH" {
			bearishScore += 2.0 * regime.Strength
		}
	} else if regime.Type == "RANGING" {
		// In ranging markets, favor mean reversion
		if currentPrice > indicators.BBMiddle {
			bearishScore += 0.5 * regime.Strength
		} else if currentPrice < indicators.BBMiddle {
			bullishScore += 0.5 * regime.Strength
		}
	}

	// Trend alignment factor (higher weight)
	if trendDirection == "BULLISH" {
		bullishScore += 1.5 * trendStrength
	} else if trendDirection == "BEARISH" {
		bearishScore += 1.5 * trendStrength
	}

	// RSI factor
	if indicators.RSI < 30 {
		bullishScore += 1.0
	} else if indicators.RSI > 70 {
		bearishScore += 1.0
	}

	// MACD factor
	if indicators.MACDHist > 0 && indicators.MACDHist > indicators.MACD*0.1 {
		bullishScore += 0.8
	} else if indicators.MACDHist < 0 && indicators.MACDHist < indicators.MACD*0.1 {
		bearishScore += 0.8
	}

	// Stochastic factor
	if indicators.Stochastic < 20 && indicators.Stochastic > indicators.StochasticSignal {
		bullishScore += 0.7 // Oversold and turning up
	} else if indicators.Stochastic > 80 && indicators.Stochastic < indicators.StochasticSignal {
		bearishScore += 0.7 // Overbought and turning down
	}

	// Pattern factors (increased weight for specific patterns)
	for _, pattern := range patterns2 {
		switch pattern {
		case "BULLISH_ENGULFING", "HAMMER", "MORNING_STAR":
			bullishScore += 1.8
		case "BEARISH_ENGULFING", "SHOOTING_STAR", "EVENING_STAR":
			bearishScore += 1.8
		case "THREE_WHITE_SOLDIERS":
			bullishScore += 2.0
		case "THREE_BLACK_CROWS":
			bearishScore += 2.0
		case "STRONG_BULLISH_MOMENTUM":
			bullishScore += 1.2
		case "STRONG_BEARISH_MOMENTUM":
			bearishScore += 1.2
		case "DOUBLE_BOTTOM":
			bullishScore += 1.5
		case "DOUBLE_TOP":
			bearishScore += 1.5
		}
	}

	// Support/Resistance proximity factor with increased weight
	if distanceToSupport > 0 && distanceToResistance < 99999.0 {
		supportFactor := math.Min(1.0, expectedMove/distanceToSupport)
		resistanceFactor := math.Min(1.0, expectedMove/distanceToResistance)

		if distanceToSupport < distanceToResistance {
			// Closer to support
			bearishScore -= supportFactor * 0.8 // Reduce bearish score near support
			bullishScore += supportFactor * 0.5 // Add bullish score near support
		} else {
			// Closer to resistance
			bullishScore -= resistanceFactor * 0.8 // Reduce bullish score near resistance
			bearishScore += resistanceFactor * 0.5 // Add bearish score near resistance
		}
	}

	// Order flow factor
	if flowDirection == "BULLISH" {
		bullishScore += 1.0
	} else if flowDirection == "BEARISH" {
		bearishScore += 1.0
	}

	// Trade signal factor
	if indicators.TradeSignal == "STRONG_BUY" {
		bullishScore += 1.5
	} else if indicators.TradeSignal == "BUY" {
		bullishScore += 0.8
	} else if indicators.TradeSignal == "STRONG_SELL" {
		bearishScore += 1.5
	} else if indicators.TradeSignal == "SELL" {
		bearishScore += 0.8
	}

	// Anomaly adjustment
	if anomaly.IsAnomaly {
		// During anomalies, reduce overall confidence
		bullishScore *= (1.0 - anomaly.AnomalyScore*0.3)
		bearishScore *= (1.0 - anomaly.AnomalyScore*0.3)
	}

	// Volatility adjustment
	confidenceMultiplier := 1.0
	if volatilityRegime == "HIGH" {
		confidenceMultiplier = 0.8 // Reduce confidence in high volatility
	} else if volatilityRegime == "LOW" {
		confidenceMultiplier = 0.9 // Slightly reduce confidence in low volatility
	}

	// Final direction decision
	direction := "NEUTRAL"
	netScore := (bullishScore - bearishScore) * confidenceMultiplier

	if netScore > 1.5 {
		direction = "UP"
	} else if netScore < -1.5 {
		direction = "DOWN"
	}

	// Confidence calculation
	confidence := "MEDIUM"
	absoluteNetScore := math.Abs(netScore)

	if absoluteNetScore > 3.0 {
		confidence = "HIGH"
	} else if absoluteNetScore < 2.0 {
		confidence = "LOW"
	}

	// Decision factors for explanation
	var factors []string

	if direction == "UP" {
		if regime.Type == "TRENDING" && regime.Direction == "BULLISH" && regime.Strength > 0.6 {
			factors = append(factors, fmt.Sprintf("Strong bullish market regime (%.1f strength)", regime.Strength))
		}

		if trendDirection == "BULLISH" && trendStrength > 0.5 {
			factors = append(factors, fmt.Sprintf("Bullish alignment across timeframes (%.1f)", trendStrength))
		}

		if indicators.RSI < 40 {
			factors = append(factors, fmt.Sprintf("Oversold RSI at %.1f", indicators.RSI))
		}

		if indicators.Stochastic < 20 && indicators.Stochastic > indicators.StochasticSignal {
			factors = append(factors, fmt.Sprintf("Stochastic turning up from oversold (%.1f)", indicators.Stochastic))
		}

		for _, pattern := range patterns2 {
			if pattern == "BULLISH_ENGULFING" || pattern == "HAMMER" || pattern == "MORNING_STAR" ||
				pattern == "THREE_WHITE_SOLDIERS" || pattern == "DOUBLE_BOTTOM" {
				factors = append(factors, "Bullish reversal pattern: "+pattern)
			}
		}

		if flowDirection == "BULLISH" {
			factors = append(factors, "Positive order flow with higher volume on up candles")
		}

		if distanceToSupport < expectedMove {
			factors = append(factors, fmt.Sprintf("Price found support at %.5f", nearestSupport))
		}

		if indicators.TradeSignal == "STRONG_BUY" || indicators.TradeSignal == "BUY" {
			factors = append(factors, "Multiple indicators aligning bullish: "+indicators.TradeSignal)
		}
	} else if direction == "DOWN" {
		if regime.Type == "TRENDING" && regime.Direction == "BEARISH" && regime.Strength > 0.6 {
			factors = append(factors, fmt.Sprintf("Strong bearish market regime (%.1f strength)", regime.Strength))
		}

		if trendDirection == "BEARISH" && trendStrength > 0.5 {
			factors = append(factors, fmt.Sprintf("Bearish alignment across timeframes (%.1f)", trendStrength))
		}

		if indicators.RSI > 60 {
			factors = append(factors, fmt.Sprintf("Overbought RSI at %.1f", indicators.RSI))
		}

		if indicators.Stochastic > 80 && indicators.Stochastic < indicators.StochasticSignal {
			factors = append(factors, fmt.Sprintf("Stochastic turning down from overbought (%.1f)", indicators.Stochastic))
		}

		for _, pattern := range patterns2 {
			if pattern == "BEARISH_ENGULFING" || pattern == "SHOOTING_STAR" || pattern == "EVENING_STAR" ||
				pattern == "THREE_BLACK_CROWS" || pattern == "DOUBLE_TOP" {
				factors = append(factors, "Bearish reversal pattern: "+pattern)
			}
		}

		if flowDirection == "BEARISH" {
			factors = append(factors, "Negative order flow with higher volume on down candles")
		}

		if distanceToResistance < expectedMove {
			factors = append(factors, fmt.Sprintf("Price rejected at resistance %.5f", nearestResistance))
		}

		if indicators.TradeSignal == "STRONG_SELL" || indicators.TradeSignal == "SELL" {
			factors = append(factors, "Multiple indicators aligning bearish: "+indicators.TradeSignal)
		}
	}

	// If we don't have at least 2 factors, add more generic ones
	// If we don't have at least 2 factors, add more generic ones
	if len(factors) < 2 {
		if direction == "UP" {
			if indicators.MACDHist > 0 {
				factors = append(factors, "Positive MACD histogram")
			}
			if currentPrice > indicators.EMA {
				factors = append(factors, "Price above EMA")
			}
			if indicators.ADX > 20 && indicators.PlusDI > indicators.MinusDI {
				factors = append(factors, "ADX shows bullish trend")
			}
			if currentPrice > indicators.BBMiddle {
				factors = append(factors, "Price above Bollinger middle band")
			}
		} else if direction == "DOWN" {
			if indicators.MACDHist < 0 {
				factors = append(factors, "Negative MACD histogram")
			}
			if currentPrice < indicators.EMA {
				factors = append(factors, "Price below EMA")
			}
			if indicators.ADX > 20 && indicators.MinusDI > indicators.PlusDI {
				factors = append(factors, "ADX shows bearish trend")
			}
			if currentPrice < indicators.BBMiddle {
				factors = append(factors, "Price below Bollinger middle band")
			}
		}
	}

	// Still need more factors? Add very generic ones
	if len(factors) < 2 {
		if direction == "UP" {
			factors = append(factors, "General market strength")
		} else if direction == "DOWN" {
			factors = append(factors, "General market weakness")
		} else {
			factors = append(factors, "Market in consolidation")
		}
	}

	for _, divergence := range divergences {
		switch divergence.Type {
		case "REGULAR":
			if divergence.Direction == "BULLISH" {
				bullishScore += 1.5 * divergence.SignalStrength

				// Добавляем в факторы для объяснения если направление совпадает
				if direction == "UP" || (netScore > 0 && direction == "NEUTRAL") {
					factors = append(factors, fmt.Sprintf("Регулярная бычья дивергенция RSI (сила %.2f)",
						divergence.SignalStrength))
				}
			} else if divergence.Direction == "BEARISH" {
				bearishScore += 1.5 * divergence.SignalStrength

				// Добавляем в факторы для объяснения если направление совпадает
				if direction == "DOWN" || (netScore < 0 && direction == "NEUTRAL") {
					factors = append(factors, fmt.Sprintf("Регулярная медвежья дивергенция RSI (сила %.2f)",
						divergence.SignalStrength))
				}
			}
		case "HIDDEN":
			// Скрытые дивергенции - сигналы продолжения тренда
			if divergence.Direction == "BULLISH" {
				bullishScore += 1.0 * divergence.SignalStrength

				if direction == "UP" || (netScore > 0 && direction == "NEUTRAL") {
					factors = append(factors, fmt.Sprintf("Скрытая бычья дивергенция RSI (сила %.2f)",
						divergence.SignalStrength))
				}
			} else if divergence.Direction == "BEARISH" {
				bearishScore += 1.0 * divergence.SignalStrength

				if direction == "DOWN" || (netScore < 0 && direction == "NEUTRAL") {
					factors = append(factors, fmt.Sprintf("Скрытая медвежья дивергенция RSI (сила %.2f)",
						divergence.SignalStrength))
				}
			}
		}
	}
	stopLossLevel := calculate.DetermineStopLoss(candles, indicators, direction)

	accountSize := 10000.0 // Примерный размер счета, можно получать из конфига
	riskPerTrade := 0.01   // 1% от счета на сделку

	// Расчет размера позиции
	positionSizing := calculate.CalculatePositionSize(
		candles[len(candles)-1].Close,
		stopLossLevel,
		accountSize,
		riskPerTrade,
	)

	// Создаем торговую рекомендацию
	tradingSuggestion := &models.TradingSuggestion{
		Direction:       direction,
		Confidence:      confidence,
		Score:           netScore,
		EntryPrice:      candles[len(candles)-1].Close,
		StopLoss:        positionSizing.StopLoss,
		TakeProfit:      positionSizing.TakeProfit,
		PositionSize:    positionSizing.PositionSize,
		RiskRewardRatio: positionSizing.RiskRewardRatio,
		AccountRisk:     positionSizing.AccountRisk * 100, // в процентах
		Factors:         factors,
	}

	// Если направление нейтральное или уверенность низкая, не рекомендуем сделку
	if direction == "NEUTRAL" || confidence == "LOW" {
		tradingSuggestion.Action = "NO_TRADE"
	} else {
		if direction == "UP" {
			tradingSuggestion.Action = "BUY"
		} else {
			tradingSuggestion.Action = "SELL"
		}
	}

	return direction, confidence, netScore, factors, tradingSuggestion

}
