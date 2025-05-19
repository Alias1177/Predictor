package analyze

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
	"github.com/rs/zerolog/log"
)

// analyzeOrderFlow анализирует динамику объема
func analyzeOrderFlow(candles []models.Candle) (string, float64) {
	// Проверка наличия данных объема
	hasVolume := true
	for _, c := range candles {
		if c.Volume == 0 {
			hasVolume = false
			break
		}
	}

	if !hasVolume {
		return "NO_VOLUME_DATA", 0
	}

	// Расчет взвешенной по объему цены
	var totalVolume int64
	var volumeWeightedPrice float64

	for i := len(candles) - 5; i < len(candles); i++ {
		volumeWeightedPrice += candles[i].Close * float64(candles[i].Volume)
		totalVolume += candles[i].Volume
	}

	if totalVolume > 0 {
		volumeWeightedPrice /= float64(totalVolume)
	}

	// Расчет тренда объема
	var upVolume, downVolume int64
	for i := len(candles) - 5; i < len(candles); i++ {
		if candles[i].Close > candles[i].Open {
			upVolume += candles[i].Volume
		} else {
			downVolume += candles[i].Volume
		}
	}

	// Расчет соотношения объемов
	volumeRatio := 0.5
	if upVolume+downVolume > 0 {
		volumeRatio = float64(upVolume) / float64(upVolume+downVolume)
	}

	// Определение направления потока объема
	flowDirection := "NEUTRAL"
	if volumeRatio > 0.65 {
		flowDirection = "BULLISH"
	} else if volumeRatio < 0.35 {
		flowDirection = "BEARISH"
	}

	return flowDirection, volumeWeightedPrice
}

// EnhancedOrderFlowAnalysis анализирует давление покупателей/продавцов с помощью Delta Volume
func EnhancedOrderFlowAnalysis(candles []models.Candle) *models.OrderFlow {
	if len(candles) < 5 {
		return &models.OrderFlow{
			Direction:       "NEUTRAL",
			Strength:        0,
			BuyingPressure:  0,
			SellingPressure: 0,
			DeltaPercentage: 0,
			IsClimaxVolume:  false,
			IsExhaustion:    false,
		}
	}

	// Расчет Delta Volume (давление покупателей vs продавцов)
	var deltaVolume float64
	var totalVolume int64
	var buyVolume, sellVolume int64

	for i := max(0, len(candles)-10); i < len(candles); i++ {
		candle := candles[i]
		volume := candle.Volume
		totalVolume += volume

		// Определяем, бычья свеча или медвежья
		bullish := bullishCandle(candle)

		// Расчет взвешенного по объему дельта
		if bullish {
			buyVolume += volume
			// Расчет позиции закрытия в диапазоне свечи
			positionFactor := 0.5
			if candle.High > candle.Low {
				positionFactor = (candle.Close - candle.Low) / (candle.High - candle.Low)
			}
			deltaVolume += float64(volume) * positionFactor
		} else {
			sellVolume += volume
			// Для медвежьих свечей - отрицательный вклад
			positionFactor := 0.5
			if candle.High > candle.Low {
				positionFactor = (candle.High - candle.Close) / (candle.High - candle.Low)
			}
			deltaVolume -= float64(volume) * positionFactor
		}
	}

	// Расчет соотношения покупок/продаж и процента дельты
	buyRatio, sellRatio := 0.5, 0.5
	if totalVolume > 0 {
		buyRatio = float64(buyVolume) / float64(totalVolume)
		sellRatio = float64(sellVolume) / float64(totalVolume)
	}

	deltaPercentage := 0.0
	if totalVolume > 0 {
		deltaPercentage = deltaVolume / float64(totalVolume)
	}

	// Определяем направление потока ордеров и силу
	direction := "NEUTRAL"
	if deltaPercentage > 0.1 {
		direction = "BULLISH"
	} else if deltaPercentage < -0.1 {
		direction = "BEARISH"
	}

	strength := math.Min(math.Abs(deltaPercentage)*5, 1.0)

	// Определяем условия пикового объема
	isClimaxVolume := false
	isExhaustion := false

	if len(candles) > 1 {
		recentVolume := candles[len(candles)-1].Volume
		prevVolumes := int64(0)
		for i := max(0, len(candles)-10); i < len(candles)-1; i++ {
			prevVolumes += candles[i].Volume
		}

		avgVolume := float64(1)
		if len(candles) > 2 {
			avgVolume = float64(prevVolumes) / float64(len(candles)-1)
		}

		volumeSpike := 1.0
		if avgVolume > 0 {
			volumeSpike = float64(recentVolume) / avgVolume
		}

		isClimaxVolume = volumeSpike > 2.5

		// Исчерпание - это скачок объема против тренда
		if isClimaxVolume {
			lastCandle := candles[len(candles)-1]
			if direction == "BULLISH" && lastCandle.Close < lastCandle.Open {
				isExhaustion = true
			} else if direction == "BEARISH" && lastCandle.Close > lastCandle.Open {
				isExhaustion = true
			}
		}
	}

	return &models.OrderFlow{
		Direction:       direction,
		Strength:        strength,
		BuyingPressure:  buyRatio,
		SellingPressure: sellRatio,
		DeltaPercentage: deltaPercentage,
		IsClimaxVolume:  isClimaxVolume,
		IsExhaustion:    isExhaustion,
	}
}

// assessVolatilityConditions анализирует волатильность рынка
func assessVolatilityConditions(candles []models.Candle) (string, float64) {
	// Расчет ATR для разных периодов
	atr5 := utils.CalculateATR(candles, 5)
	atr20 := utils.CalculateATR(candles, 20)

	// Расчет соотношения волатильности
	volatilityRatio := 1.0
	if atr20 > 0 {
		volatilityRatio = atr5 / atr20
	}

	// Определение режима волатильности
	volatilityRegime := "NORMAL"
	if volatilityRatio > 1.5 {
		volatilityRegime = "HIGH"
	} else if volatilityRatio < 0.7 {
		volatilityRegime = "LOW"
	}

	// Расчет недавнего диапазона
	var highestHigh, lowestLow float64
	for i := len(candles) - 10; i < len(candles); i++ {
		if i == len(candles)-10 || candles[i].High > highestHigh {
			highestHigh = candles[i].High
		}
		if i == len(candles)-10 || candles[i].Low < lowestLow {
			lowestLow = candles[i].Low
		}
	}

	// Расчет ожидаемого движения для выбранного таймфрейма
	expectedMove := atr5

	return volatilityRegime, expectedMove
}

// Проверка, является ли свеча растущей
func bullishCandle(candle models.Candle) bool {
	return candle.Close > candle.Open
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// analyzeMarketSentiment анализирует настроения рынка
func analyzeMarketSentiment(candles []models.Candle) *models.MarketSentiment {
	sentiment := &models.MarketSentiment{}

	// Анализ индекса страха и жадности
	sentiment.FearGreedIndex = calculateFearGreedIndex(candles)

	// Анализ настроений по объему
	sentiment.VolumeSentiment = calculateVolumeSentiment(candles)

	// Определение настроения рынка
	sentiment.MarketMood = determineMarketMood(sentiment.FearGreedIndex, sentiment.VolumeSentiment)

	// Определение настроения по волатильности
	sentiment.VolatilityMood = determineVolatilityMood(sentiment.FearGreedIndex)

	return sentiment
}

// calculateFearGreedIndex рассчитывает индекс страха и жадности
func calculateFearGreedIndex(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 50.0
	}

	// Расчет волатильности
	volatility := utils.CalculateATR(candles, 14)

	// Расчет тренда
	var upMoves, downMoves float64
	for i := 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			upMoves += change
		} else {
			downMoves += math.Abs(change)
		}
	}

	// Расчет соотношения движений
	momentum := 0.0
	if downMoves > 0 {
		momentum = upMoves / downMoves
	}

	// Нормализация волатильности
	volatilityScore := 1.0 - math.Min(volatility/candles[len(candles)-1].Close, 1.0)

	// Расчет индекса (0-100)
	index := (momentum*0.6 + volatilityScore*0.4) * 50

	// Ограничение диапазона
	return math.Max(0, math.Min(100, index))
}

// calculateVolumeSentiment рассчитывает настроения по объему
func calculateVolumeSentiment(candles []models.Candle) float64 {
	if len(candles) < 10 {
		return 0.0
	}

	var upVolume, downVolume int64
	for i := len(candles) - 10; i < len(candles); i++ {
		if candles[i].Close > candles[i].Open {
			upVolume += candles[i].Volume
		} else {
			downVolume += candles[i].Volume
		}
	}

	totalVolume := upVolume + downVolume
	if totalVolume == 0 {
		return 0.0
	}

	// Нормализация от -1 до 1
	return (float64(upVolume-downVolume) / float64(totalVolume))
}

// determineMarketMood определяет настроение рынка
func determineMarketMood(fearGreedIndex, volumeSentiment float64) string {
	// Взвешенная оценка
	score := (fearGreedIndex/100.0)*0.6 + (volumeSentiment+1.0)/2.0*0.4

	switch {
	case score > 0.7:
		return "bullish"
	case score < 0.3:
		return "bearish"
	default:
		return "neutral"
	}
}

// determineVolatilityMood определяет настроение по волатильности
func determineVolatilityMood(fearGreedIndex float64) string {
	switch {
	case fearGreedIndex > 70 || fearGreedIndex < 30:
		return "high"
	case fearGreedIndex > 60 || fearGreedIndex < 40:
		return "medium"
	default:
		return "low"
	}
}

// analyzeCorrelations анализирует корреляции
func analyzeCorrelations(candles []models.Candle, marketCandles []models.Candle) *models.CorrelationAnalysis {
	analysis := &models.CorrelationAnalysis{
		AssetCorrelations: make(map[string]float64),
	}

	// Расчет корреляции с рынком
	analysis.MarketCorrelation = calculateCorrelation(
		extractPrices(candles),
		extractPrices(marketCandles),
	)

	// Расчет корреляции с волатильностью
	analysis.VolatilityCorrelation = calculateVolatilityCorrelation(candles)

	// Расчет корреляции с объемом
	analysis.VolumeCorrelation = calculateVolumeCorrelation(candles)

	return analysis
}

// calculateCorrelation рассчитывает корреляцию между двумя наборами данных
func calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}

	// Расчет средних значений
	var sumX, sumY float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / float64(len(x))
	meanY := sumY / float64(len(y))

	// Расчет ковариации и стандартных отклонений
	var cov, varX, varY float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		cov += dx * dy
		varX += dx * dx
		varY += dy * dy
	}

	// Расчет корреляции
	if varX == 0 || varY == 0 {
		return 0.0
	}
	return cov / math.Sqrt(varX*varY)
}

// calculateVolatilityCorrelation рассчитывает корреляцию с волатильностью
func calculateVolatilityCorrelation(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	prices := extractPrices(candles)
	volatilities := make([]float64, len(candles))

	for i := 1; i < len(candles); i++ {
		volatilities[i] = math.Abs(candles[i].Close - candles[i-1].Close)
	}

	return calculateCorrelation(prices, volatilities)
}

// calculateVolumeCorrelation рассчитывает корреляцию с объемом
func calculateVolumeCorrelation(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	prices := extractPrices(candles)
	volumes := make([]float64, len(candles))

	for i := range candles {
		volumes[i] = float64(candles[i].Volume)
	}

	return calculateCorrelation(prices, volumes)
}

// extractPrices извлекает цены закрытия из свечей
func extractPrices(candles []models.Candle) []float64 {
	prices := make([]float64, len(candles))
	for i, c := range candles {
		prices[i] = c.Close
	}
	return prices
}

// analyzeLiquidity анализирует ликвидность
func analyzeLiquidity(candles []models.Candle) *models.LiquidityAnalysis {
	analysis := &models.LiquidityAnalysis{}

	// Расчет спреда
	analysis.BidAskSpread = calculateBidAskSpread(candles)

	// Расчет глубины стакана
	analysis.OrderBookDepth = calculateOrderBookDepth(candles)

	// Расчет профиля объема
	analysis.VolumeProfile = calculateVolumeProfile(candles)

	// Расчет влияния на рынок
	analysis.MarketImpact = calculateMarketImpact(candles)

	// Расчет общего скора ликвидности
	analysis.LiquidityScore = calculateLiquidityScore(candles)

	return analysis
}

// calculateBidAskSpread рассчитывает спред между bid и ask
func calculateBidAskSpread(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalSpread float64
	count := 0

	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].High > candles[i].Low {
			spread := (candles[i].High - candles[i].Low) / candles[i].Close
			totalSpread += spread
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return totalSpread / float64(count)
}

// calculateOrderBookDepth рассчитывает глубину стакана
func calculateOrderBookDepth(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalVolume int64
	for i := len(candles) - 20; i < len(candles); i++ {
		totalVolume += candles[i].Volume
	}

	avgVolume := float64(totalVolume) / 20.0
	price := candles[len(candles)-1].Close

	// Нормализация глубины стакана
	return math.Min(avgVolume/price, 1.0)
}

// calculateVolumeProfile рассчитывает профиль объема
func calculateVolumeProfile(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var upVolume, downVolume int64
	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].Close > candles[i].Open {
			upVolume += candles[i].Volume
		} else {
			downVolume += candles[i].Volume
		}
	}

	totalVolume := upVolume + downVolume
	if totalVolume == 0 {
		return 0.0
	}

	// Нормализация профиля объема
	return float64(upVolume) / float64(totalVolume)
}

// calculateMarketImpact рассчитывает влияние на рынок
func calculateMarketImpact(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalImpact float64
	count := 0

	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].Volume > 0 {
			impact := math.Abs(candles[i].Close-candles[i].Open) / float64(candles[i].Volume)
			totalImpact += impact
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return totalImpact / float64(count)
}

// calculateLiquidityScore рассчитывает оценку ликвидности
func calculateLiquidityScore(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalVolume float64
	for _, candle := range candles {
		totalVolume += float64(candle.Volume)
	}

	avgVolume := totalVolume / float64(len(candles))
	return math.Min(avgVolume/1000000, 1.0) // Нормализация к 1.0
}

// analyzeVolume анализирует объемы
func analyzeVolume(candles []models.Candle) *models.VolumeAnalysis {
	analysis := &models.VolumeAnalysis{}

	// Анализ тренда объема
	analysis.VolumeTrend = determineVolumeTrend(candles)

	// Расчет силы объема
	analysis.VolumeStrength = calculateVolumeStrength(candles)

	// Расчет профиля объема
	analysis.VolumeProfile = calculateVolumeProfile(candles)

	// Расчет дисбаланса объема
	analysis.VolumeImbalance = calculateVolumeImbalance(candles)

	// Определение кластеров объема
	analysis.VolumeClusters = identifyVolumeClusters(candles)

	return analysis
}

// determineVolumeTrend определяет тренд объема
func determineVolumeTrend(candles []models.Candle) string {
	if len(candles) < 20 {
		return "neutral"
	}

	var recentVolume, prevVolume int64
	for i := len(candles) - 10; i < len(candles); i++ {
		recentVolume += candles[i].Volume
	}
	for i := len(candles) - 20; i < len(candles)-10; i++ {
		prevVolume += candles[i].Volume
	}

	volumeChange := float64(recentVolume-prevVolume) / float64(prevVolume)
	switch {
	case volumeChange > 0.2:
		return "increasing"
	case volumeChange < -0.2:
		return "decreasing"
	default:
		return "stable"
	}
}

// calculateVolumeStrength рассчитывает силу объема
func calculateVolumeStrength(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var sumVolume float64
	for _, candle := range candles {
		sumVolume += float64(candle.Volume)
	}

	avgVolume := sumVolume / float64(len(candles))
	lastVolume := float64(candles[len(candles)-1].Volume)

	return math.Min(lastVolume/avgVolume, 1.0)
}

// calculateVolumeImbalance рассчитывает дисбаланс объема
func calculateVolumeImbalance(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var buyVolume, sellVolume int64
	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].Close > candles[i].Open {
			buyVolume += candles[i].Volume
		} else {
			sellVolume += candles[i].Volume
		}
	}

	totalVolume := buyVolume + sellVolume
	if totalVolume == 0 {
		return 0.0
	}

	// Нормализация дисбаланса от -1 до 1
	return float64(buyVolume-sellVolume) / float64(totalVolume)
}

// identifyVolumeClusters определяет кластеры объема
func identifyVolumeClusters(candles []models.Candle) []models.VolumeCluster {
	if len(candles) < 20 {
		return nil
	}

	clusters := make([]models.VolumeCluster, 0)
	priceRange := candles[len(candles)-1].High - candles[len(candles)-1].Low
	step := priceRange / 10.0

	for i := 0; i < 10; i++ {
		priceLevel := candles[len(candles)-1].Low + float64(i)*step
		var volume int64
		var direction string

		for j := len(candles) - 20; j < len(candles); j++ {
			if candles[j].Close >= priceLevel && candles[j].Close < priceLevel+step {
				volume += candles[j].Volume
				if candles[j].Close > candles[j].Open {
					direction = "buy"
				} else {
					direction = "sell"
				}
			}
		}

		if volume > 0 {
			clusters = append(clusters, models.VolumeCluster{
				Price:     priceLevel,
				Volume:    volume,
				Direction: direction,
			})
		}
	}

	return clusters
}

// analyzeMicrostructure анализирует микроструктуру рынка
func analyzeMicrostructure(candles []models.Candle) *models.MicrostructureAnalysis {
	analysis := &models.MicrostructureAnalysis{}

	// Анализ потока ордеров
	analysis.OrderFlow = analyzeOrderFlowMicrostructure(candles)

	// Анализ влияния на цену
	analysis.PriceImpact = analyzePriceImpact(candles)

	// Анализ качества рынка
	analysis.MarketQuality = analyzeMarketQuality(candles)

	// Анализ торговой активности
	analysis.TradingActivity = analyzeTradingActivity(candles)

	return analysis
}

// analyzeOrderFlowMicrostructure анализирует поток ордеров для микроструктуры
func analyzeOrderFlowMicrostructure(candles []models.Candle) models.OrderFlowAnalysis {
	analysis := models.OrderFlowAnalysis{}

	if len(candles) < 20 {
		return analysis
	}

	var buyVolume, sellVolume int64
	var largeOrders int

	for i := len(candles) - 20; i < len(candles); i++ {
		volume := candles[i].Volume
		if volume > 0 {
			if candles[i].Close > candles[i].Open {
				buyVolume += volume
			} else {
				sellVolume += volume
			}

			// Определение крупных ордеров
			if float64(volume) > float64(candles[i].Close)*1000 {
				largeOrders++
			}
		}
	}

	totalVolume := buyVolume + sellVolume
	if totalVolume > 0 {
		analysis.BuyPressure = float64(buyVolume) / float64(totalVolume)
		analysis.SellPressure = float64(sellVolume) / float64(totalVolume)
		analysis.NetFlow = analysis.BuyPressure - analysis.SellPressure
		analysis.FlowImbalance = math.Abs(analysis.NetFlow)
	}

	analysis.LargeOrders = largeOrders
	return analysis
}

// analyzePriceImpact анализирует влияние на цену
func analyzePriceImpact(candles []models.Candle) models.PriceImpactAnalysis {
	analysis := models.PriceImpactAnalysis{}

	if len(candles) < 20 {
		return analysis
	}

	var immediateImpact, permanentImpact float64
	var impactCount int

	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].Volume > 0 {
			// Мгновенное влияние
			immediateImpact += math.Abs(candles[i].Close - candles[i].Open)

			// Постоянное влияние
			if i > 0 {
				permanentImpact += math.Abs(candles[i].Close - candles[i-1].Close)
			}

			impactCount++
		}
	}

	if impactCount > 0 {
		analysis.ImmediateImpact = immediateImpact / float64(impactCount)
		analysis.PermanentImpact = permanentImpact / float64(impactCount)
		analysis.ImpactDecay = analysis.PermanentImpact / analysis.ImmediateImpact
	}

	// Расчет эластичности цены
	priceChanges := make([]float64, 0)
	volumeChanges := make([]float64, 0)

	for i := 1; i < len(candles); i++ {
		if candles[i].Volume > 0 && candles[i-1].Volume > 0 {
			priceChange := (candles[i].Close - candles[i-1].Close) / candles[i-1].Close
			volumeChange := float64(candles[i].Volume-candles[i-1].Volume) / float64(candles[i-1].Volume)

			priceChanges = append(priceChanges, priceChange)
			volumeChanges = append(volumeChanges, volumeChange)
		}
	}

	if len(priceChanges) > 0 {
		analysis.PriceElasticity = calculateCorrelation(priceChanges, volumeChanges)
	}

	return analysis
}

// analyzeMarketQuality анализирует качество рынка
func analyzeMarketQuality(candles []models.Candle) models.MarketQualityMetrics {
	metrics := models.MarketQualityMetrics{}

	if len(candles) < 20 {
		return metrics
	}

	// Расчет эффективности рынка
	var priceChanges []float64
	for i := 1; i < len(candles); i++ {
		priceChanges = append(priceChanges, math.Abs(candles[i].Close-candles[i-1].Close))
	}
	metrics.Efficiency = 1.0 - calculateCorrelation(priceChanges, make([]float64, len(priceChanges)))

	// Расчет устойчивости рынка
	var recoveryCount int
	for i := 1; i < len(candles)-1; i++ {
		if math.Abs(candles[i].Close-candles[i-1].Close) > 0.01 {
			if math.Abs(candles[i+1].Close-candles[i].Close) < 0.005 {
				recoveryCount++
			}
		}
	}
	metrics.Resilience = float64(recoveryCount) / float64(len(candles)-2)

	// Расчет фрагментации рынка
	var spreadSum float64
	for i := len(candles) - 20; i < len(candles); i++ {
		spreadSum += (candles[i].High - candles[i].Low) / candles[i].Close
	}
	metrics.Fragmentation = spreadSum / 20.0

	// Расчет прозрачности рынка
	var volumeSum int64
	for i := len(candles) - 20; i < len(candles); i++ {
		volumeSum += candles[i].Volume
	}
	metrics.Transparency = 1.0 - math.Min(float64(volumeSum)/1000000.0, 1.0)

	return metrics
}

// analyzeTradingActivity анализирует торговую активность
func analyzeTradingActivity(candles []models.Candle) models.TradingActivityMetrics {
	metrics := models.TradingActivityMetrics{}

	if len(candles) < 20 {
		return metrics
	}

	// Расчет частоты сделок
	var tradeCount int
	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].Volume > 0 {
			tradeCount++
		}
	}
	metrics.TradeFrequency = float64(tradeCount) / 20.0

	// Расчет размера сделок
	var totalVolume int64
	for i := len(candles) - 20; i < len(candles); i++ {
		totalVolume += candles[i].Volume
	}
	metrics.TradeSize = float64(totalVolume) / float64(tradeCount)

	// Расчет стоимости сделок
	var totalValue float64
	for i := len(candles) - 20; i < len(candles); i++ {
		totalValue += float64(candles[i].Volume) * candles[i].Close
	}
	metrics.TradeValue = totalValue / float64(tradeCount)

	// Расчет активных трейдеров (оценка)
	metrics.ActiveTraders = int(math.Sqrt(float64(totalVolume) / 1000.0))

	return metrics
}

// NewsAnalysis представляет анализ новостей
type NewsAnalysis struct {
	Sentiment    float64            `json:"sentiment"`     // Общий сентимент (-1 до 1)
	Impact       float64            `json:"impact"`        // Влияние на рынок (0-1)
	Relevance    float64            `json:"relevance"`     // Релевантность (0-1)
	NewsCount    int                `json:"news_count"`    // Количество новостей
	TopNews      []models.NewsItem  `json:"top_news"`      // Важные новости
	MarketImpact map[string]float64 `json:"market_impact"` // Влияние на разные аспекты рынка
}

// NewsItem представляет отдельную новость
type NewsItem struct {
	Title       string    `json:"title"`        // Заголовок
	Content     string    `json:"content"`      // Содержание
	Sentiment   float64   `json:"sentiment"`    // Сентимент (-1 до 1)
	Impact      float64   `json:"impact"`       // Влияние (0-1)
	PublishedAt time.Time `json:"published_at"` // Время публикации
	Symbol      string    `json:"symbol"`       // Символ
}

// FundamentalAnalysis представляет фундаментальный анализ
type FundamentalAnalysis struct {
	EconomicIndicators map[string]float64 `json:"economic_indicators"` // Экономические индикаторы
	MarketConditions   map[string]float64 `json:"market_conditions"`   // Рыночные условия
	RiskFactors        map[string]float64 `json:"risk_factors"`        // Факторы риска
	MarketRegime       string             `json:"market_regime"`       // Режим рынка
	RegimeStrength     float64            `json:"regime_strength"`     // Сила режима (0-1)
}

// fetchNews получает новости через API
func fetchNews(_ string) ([]models.NewsItem, error) {
	apiKey := "cv9mclpr01qpd9s82rngcv9mclpr01qpd9s82ro0"
	apiUrl := "https://finnhub.io/api/v1/news"

	// Формируем URL с параметрами
	url := fmt.Sprintf("%s?category=forex&token=%s", apiUrl, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var news []models.NewsItem
	if err := json.NewDecoder(resp.Body).Decode(&news); err != nil {
		return nil, err
	}

	return news, nil
}

// analyzeNews анализирует новости и их влияние на рынок
func analyzeNews(symbol string, _ string) *models.NewsAnalysis {
	analysis := &models.NewsAnalysis{
		MarketImpact: make(map[string]float64),
	}

	// Получение новостей через API
	news, err := fetchNews(symbol)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching news")
		return analysis
	}

	// Анализ каждой новости
	var totalSentiment, totalImpact float64
	for _, item := range news {
		// Устанавливаем символ для новости
		item.Symbol = symbol

		// Анализ сентимента
		sentiment := analyzeNewsSentiment(item)
		totalSentiment += sentiment

		// Оценка влияния
		impact := calculateNewsImpact(item, symbol)
		totalImpact += impact

		// Добавление важных новостей
		if impact > 0.7 {
			analysis.TopNews = append(analysis.TopNews, item)
		}
	}

	// Расчет общих метрик
	if len(news) > 0 {
		analysis.Sentiment = totalSentiment / float64(len(news))
		analysis.Impact = totalImpact / float64(len(news))
		analysis.NewsCount = len(news)
	}

	// Анализ влияния на разные аспекты рынка
	analysis.MarketImpact = calculateNewsMarketImpact(news)

	return analysis
}

// analyzeNewsSentiment анализирует сентимент новости
func analyzeNewsSentiment(item models.NewsItem) float64 {
	// Простой анализ на основе ключевых слов
	positiveWords := []string{"bullish", "growth", "increase", "positive", "gain", "up", "rise", "strong"}
	negativeWords := []string{"bearish", "decline", "decrease", "negative", "loss", "down", "fall", "weak"}

	text := strings.ToLower(item.Title + " " + item.Content)

	var score float64
	for _, word := range positiveWords {
		if strings.Contains(text, word) {
			score += 0.2
		}
	}
	for _, word := range negativeWords {
		if strings.Contains(text, word) {
			score -= 0.2
		}
	}

	return math.Max(-1, math.Min(1, score))
}

// calculateNewsImpact рассчитывает влияние новости
func calculateNewsImpact(item models.NewsItem, symbol string) float64 {
	sentiment := analyzeNewsSentiment(item)

	// Влияние уменьшается со временем
	timeSincePublished := time.Since(item.PublishedAt)
	timeFactor := math.Exp(-timeSincePublished.Hours() / 24.0) // Экспоненциальное затухание

	// Учитываем релевантность для конкретного символа
	relevanceFactor := 1.0
	if symbol != "" && strings.Contains(strings.ToLower(item.Title), strings.ToLower(symbol)) {
		relevanceFactor = 1.5
	}

	return math.Abs(sentiment) * timeFactor * relevanceFactor
}

// calculateNewsMarketImpact рассчитывает влияние новостей на рынок
func calculateNewsMarketImpact(news []models.NewsItem) map[string]float64 {
	impact := make(map[string]float64)

	// Анализируем влияние на разные аспекты рынка
	var sentimentImpact, volatilityImpact, liquidityImpact float64

	for _, item := range news {
		sentiment := analyzeNewsSentiment(item)
		itemImpact := calculateNewsImpact(item, item.Symbol)

		// Влияние на настроения
		sentimentImpact += sentiment * itemImpact

		// Влияние на волатильность
		if math.Abs(sentiment) > 0.7 {
			volatilityImpact += itemImpact
		}

		// Влияние на ликвидность
		if strings.Contains(strings.ToLower(item.Title), "liquidity") {
			liquidityImpact += itemImpact
		}
	}

	// Нормализация влияния
	if len(news) > 0 {
		sentimentImpact /= float64(len(news))
		volatilityImpact /= float64(len(news))
		liquidityImpact /= float64(len(news))
	}

	impact["sentiment"] = math.Max(-1, math.Min(1, sentimentImpact))
	impact["volatility"] = math.Min(1, volatilityImpact)
	impact["liquidity"] = math.Min(1, liquidityImpact)

	return impact
}

// analyzeFundamentals анализирует фундаментальные факторы
func analyzeFundamentals(symbol string, candles []models.Candle) *models.FundamentalAnalysis {
	analysis := &models.FundamentalAnalysis{
		EconomicIndicators: make(map[string]float64),
		MarketConditions:   make(map[string]float64),
		RiskFactors:        make(map[string]float64),
	}

	// Определение режима рынка
	analysis.MarketRegime = determineMarketRegime(candles)
	analysis.RegimeStrength = calculateRegimeStrength(candles)

	// Анализ экономических индикаторов
	analysis.EconomicIndicators = analyzeEconomicIndicators(symbol)

	// Анализ рыночных условий
	analysis.MarketConditions = analyzeMarketConditions(candles)

	// Анализ факторов риска
	analysis.RiskFactors = analyzeRiskFactors(symbol, candles)

	return analysis
}

// analyzeMarketConditions анализирует рыночные условия
func analyzeMarketConditions(candles []models.Candle) map[string]float64 {
	conditions := make(map[string]float64)

	// Анализ ликвидности
	conditions["liquidity"] = calculateLiquidityScore(candles)

	// Анализ глубины рынка
	conditions["market_depth"] = calculateMarketDepth(candles)

	// Анализ спреда
	conditions["spread"] = calculateSpread(candles)

	// Анализ волатильности
	conditions["volatility"] = calculateVolatility(candles)

	return conditions
}

// calculateMarketDepth рассчитывает глубину рынка
func calculateMarketDepth(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalVolume float64
	for i := len(candles) - 20; i < len(candles); i++ {
		totalVolume += float64(candles[i].Volume)
	}

	avgVolume := totalVolume / 20.0
	return math.Min(avgVolume/1000000, 1.0)
}

// calculateSpread рассчитывает спред
func calculateSpread(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	var totalSpread float64
	count := 0

	for i := len(candles) - 20; i < len(candles); i++ {
		if candles[i].High > candles[i].Low {
			spread := (candles[i].High - candles[i].Low) / candles[i].Close
			totalSpread += spread
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return totalSpread / float64(count)
}

// calculateVolatilityRisk рассчитывает риск волатильности
func calculateVolatilityRisk(candles []models.Candle) float64 {
	volatility := calculateVolatility(candles)
	return math.Min(volatility*5, 1.0)
}

// determineDirection определяет направление движения на основе всех анализов
func determineDirection(sentiment *models.MarketSentiment, _ *models.CorrelationAnalysis,
	lLiquidity *models.LiquidityAnalysis, volume *models.VolumeAnalysis,
	microstructure *models.MicrostructureAnalysis, news *models.NewsAnalysis,
	fundamentals *models.FundamentalAnalysis) string {

	var bullishScore, bearishScore float64

	// Анализ настроений
	switch sentiment.MarketMood {
	case "bullish":
		bullishScore += 0.2
	case "bearish":
		bearishScore += 0.2
	}

	// Анализ объемов
	if volume.VolumeImbalance > 0.2 {
		bullishScore += 0.15
	} else if volume.VolumeImbalance < -0.2 {
		bearishScore += 0.15
	}

	// Анализ ликвидности
	if lLiquidity != nil && lLiquidity.LiquidityScore > 0.7 {
		bullishScore += 0.1
	} else if lLiquidity != nil && lLiquidity.LiquidityScore < 0.3 {
		bearishScore += 0.1
	}

	// Анализ микроструктуры
	if microstructure.OrderFlow.NetFlow > 0.2 {
		bullishScore += 0.15
	} else if microstructure.OrderFlow.NetFlow < -0.2 {
		bearishScore += 0.15
	}

	// Анализ новостей
	if news.Sentiment > 0.2 {
		bullishScore += 0.2
	} else if news.Sentiment < -0.2 {
		bearishScore += 0.2
	}

	// Анализ фундаментальных факторов
	if fundamentals.MarketRegime == "TRENDING_UP" {
		bullishScore += 0.2
	} else if fundamentals.MarketRegime == "TRENDING_DOWN" {
		bearishScore += 0.2
	}

	// Определение направления
	if bullishScore > bearishScore+0.2 {
		return "bullish"
	} else if bearishScore > bullishScore+0.2 {
		return "bearish"
	}
	return "neutral"
}

// calculateConfidence рассчитывает уверенность в предсказании
func calculateConfidence(sentiment *models.MarketSentiment, correlations *models.CorrelationAnalysis,
	lLiquidity *models.LiquidityAnalysis, volume *models.VolumeAnalysis,
	microstructure *models.MicrostructureAnalysis, news *models.NewsAnalysis,
	fundamentals *models.FundamentalAnalysis) float64 {

	var confidence float64

	// Уверенность на основе настроений
	confidence += math.Abs(sentiment.FearGreedIndex-50) / 50.0 * 0.15

	// Уверенность на основе объемов
	confidence += math.Abs(volume.VolumeImbalance) * 0.15

	// Уверенность на основе ликвидности
	if lLiquidity != nil {
		confidence += lLiquidity.LiquidityScore * 0.15
	}

	// Уверенность на основе микроструктуры
	confidence += math.Abs(microstructure.OrderFlow.NetFlow) * 0.15

	// Уверенность на основе корреляций
	confidence += math.Abs(correlations.MarketCorrelation) * 0.1

	// Уверенность на основе новостей
	confidence += math.Abs(news.Sentiment) * 0.15

	// Уверенность на основе фундаментальных факторов
	confidence += fundamentals.RegimeStrength * 0.15

	return math.Min(confidence, 1.0)
}

// calculateVolatility рассчитывает волатильность
func calculateVolatility(candles []models.Candle) float64 {
	if len(candles) < 2 {
		return 0.0
	}

	// Расчет дневных доходностей
	returns := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		returns[i-1] = (candles[i].Close - candles[i-1].Close) / candles[i-1].Close
	}

	// Расчет среднего значения
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// Расчет дисперсии
	var variance float64
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns))

	// Годовая волатильность (предполагаем 252 торговых дня)
	annualizedVol := math.Sqrt(variance) * math.Sqrt(252.0)

	// Нормализация волатильности для 5-минутного таймфрейма
	return annualizedVol * math.Sqrt(5.0/1440.0) // 5 минут / 1440 минут в дне
}

// calculateTrend рассчитывает тренд
func calculateTrend(candles []models.Candle) float64 {
	if len(candles) < 2 {
		return 0.0
	}

	// Используем линейную регрессию для определения тренда
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(candles))

	for i, candle := range candles {
		x := float64(i)
		y := candle.Close
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Расчет наклона линии тренда
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	// Нормализация тренда
	return slope / candles[0].Close
}

// calculateTrendStrength рассчитывает силу тренда
func calculateTrendStrength(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	// Расчет ADX (Average Directional Index)
	var plusDM, minusDM, plusDI, minusDI float64
	var tr float64

	for i := 1; i < len(candles); i++ {
		// True Range
		tr = math.Max(
			candles[i].High-candles[i].Low,
			math.Max(
				math.Abs(candles[i].High-candles[i-1].Close),
				math.Abs(candles[i].Low-candles[i-1].Close),
			),
		)

		// Directional Movement
		upMove := candles[i].High - candles[i-1].High
		downMove := candles[i-1].Low - candles[i].Low

		if upMove > downMove && upMove > 0 {
			plusDM += upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM += downMove
		}
	}

	// Расчет Directional Indicators
	plusDI = (plusDM / tr) * 100
	minusDI = (minusDM / tr) * 100

	// Расчет ADX
	dx := math.Abs(plusDI-minusDI) / (plusDI + minusDI) * 100

	// Нормализация силы тренда
	return math.Min(dx/100, 1.0)
}

// calculateRegimeStrength рассчитывает силу текущего режима
func calculateRegimeStrength(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0.0
	}

	// Расчет силы тренда
	trendStrength := calculateTrendStrength(candles)

	// Расчет силы волатильности
	volatility := calculateVolatility(candles)
	volatilityStrength := math.Min(volatility*10, 1.0)

	// Расчет силы объема
	volumeStrength := calculateVolumeStrength(candles)

	// Расчет консистентности движения
	var consistency float64
	var upMoves, downMoves int
	for i := 1; i < len(candles); i++ {
		if candles[i].Close > candles[i-1].Close {
			upMoves++
		} else if candles[i].Close < candles[i-1].Close {
			downMoves++
		}
	}
	totalMoves := upMoves + downMoves
	if totalMoves > 0 {
		consistency = math.Abs(float64(upMoves-downMoves)) / float64(totalMoves)
	}

	// Взвешенная сумма всех факторов
	strength := trendStrength*0.3 +
		volatilityStrength*0.2 +
		volumeStrength*0.2 +
		consistency*0.3

	return math.Min(strength, 1.0)
}

// fetchEconomicData получает экономические данные
func fetchEconomicData(_ string) (map[string]float64, error) {
	// TODO: В будущем можно добавить реальный API для получения экономических данных
	// Сейчас возвращаем тестовые данные

	data := make(map[string]float64)

	// Базовые экономические индикаторы
	data["gdp_growth"] = 2.5
	data["inflation"] = 3.2
	data["interest_rate"] = 4.5
	data["unemployment"] = 5.1

	// Рыночные индикаторы
	data["market_cap"] = 1000000.0
	data["pe_ratio"] = 15.5
	data["dividend_yield"] = 2.1

	// Технические индикаторы
	data["volatility"] = 0.8
	data["momentum"] = 0.6
	data["trend_strength"] = 0.7

	return data, nil
}

// normalizeIndicator нормализует значение индикатора
func normalizeIndicator(value float64) float64 {
	// Определяем диапазоны для разных типов индикаторов
	const (
		minValue = -100.0
		maxValue = 100.0
	)

	// Ограничиваем значение в допустимом диапазоне
	normalized := math.Max(minValue, math.Min(maxValue, value))

	// Нормализуем к диапазону 0-1
	return (normalized - minValue) / (maxValue - minValue)
}

// calculateLiquidityRisk рассчитывает риск ликвидности
func calculateLiquidityRisk(candles []models.Candle) float64 {
	score := calculateLiquidityScore(candles)
	return 1.0 - score
}

// calculateCorrelationRisk рассчитывает риск корреляции
func calculateCorrelationRisk(_ string) float64 {
	// TODO: В будущем можно добавить реальный API для получения исторических данных
	// Сейчас используем фиксированные значения для тестирования

	// Базовые корреляции с основными активами
	correlations := map[string]float64{
		"SPY": 0.65,  // S&P 500
		"QQQ": 0.60,  // NASDAQ
		"GLD": 0.30,  // Gold
		"TLT": -0.40, // Treasury Bonds
		"VIX": -0.50, // Volatility Index
	}

	// Расчет взвешенного риска корреляции
	var totalRisk float64
	var totalWeight float64

	for _, correlation := range correlations {
		// Вес зависит от абсолютного значения корреляции
		weight := math.Abs(correlation)
		totalWeight += weight

		// Риск увеличивается с ростом абсолютной корреляции
		risk := math.Abs(correlation) * weight
		totalRisk += risk
	}

	if totalWeight == 0 {
		return 0.0
	}

	// Нормализация риска
	return totalRisk / totalWeight
}

// calculateNewsRisk рассчитывает риск новостей
func calculateNewsRisk(symbol string) float64 {
	news, err := fetchNews(symbol)
	if err != nil {
		return 0.0
	}

	var totalImpact float64
	for _, item := range news {
		item.Symbol = symbol
		impact := calculateNewsImpact(item, symbol)
		totalImpact += impact
	}

	if len(news) == 0 {
		return 0.0
	}

	return math.Min(totalImpact/float64(len(news)), 1.0)
}

// determineMarketRegime определяет текущий режим рынка
func determineMarketRegime(candles []models.Candle) string {
	if len(candles) < 20 {
		return "NEUTRAL"
	}

	// Расчет адаптивных пороговых значений на основе исторических данных
	historicalVolatilities := make([]float64, 0)
	historicalTrends := make([]float64, 0)
	historicalVolumes := make([]float64, 0)

	windowSize := 20
	for i := windowSize; i < len(candles); i++ {
		window := candles[i-windowSize : i]

		// Волатильность
		vol := calculateVolatility(window)
		historicalVolatilities = append(historicalVolatilities, vol)

		// Тренд
		trend := calculateTrend(window)
		historicalTrends = append(historicalTrends, trend)

		// Объем
		volStrength := calculateVolumeStrength(window)
		historicalVolumes = append(historicalVolumes, volStrength)
	}

	// Расчет средних значений и стандартных отклонений
	meanVol, stdVol := calculateMeanStd(historicalVolatilities)
	meanTrend, stdTrend := calculateMeanStd(historicalTrends)
	meanVolume, stdVolume := calculateMeanStd(historicalVolumes)

	// Текущие значения
	currentVol := calculateVolatility(candles[len(candles)-windowSize:])
	currentTrend := calculateTrend(candles[len(candles)-windowSize:])
	currentVolume := calculateVolumeStrength(candles[len(candles)-windowSize:])

	// Адаптивные пороговые значения
	volatilityThreshold := meanVol + stdVol*0.7
	trendThreshold := math.Max(0.003, math.Abs(meanTrend)+stdTrend*0.7)
	volumeThreshold := meanVolume + stdVolume*0.5

	// 1. VOLATILE_BULLISH/BEARISH: ещё более чувствительный порог
	if currentVol > volatilityThreshold*0.7 {
		if math.Abs(currentTrend) > trendThreshold*0.7 {
			if currentTrend > 0 {
				return "VOLATILE_BULLISH"
			}
			return "VOLATILE_BEARISH"
		}
		return "VOLATILE"
	}

	// 2. ACCUMULATION/DISTRIBUTION: только если волатильность не выше среднего + 0.3*std
	if currentVolume > volumeThreshold*0.85 && currentVol < meanVol+stdVol*0.3 {
		if currentTrend > trendThreshold*0.7 {
			return "ACCUMULATION"
		} else if currentTrend < -trendThreshold*0.7 {
			return "DISTRIBUTION"
		}
	}

	// 3. TRENDING
	if math.Abs(currentTrend) > trendThreshold {
		if currentTrend > 0 {
			return "TRENDING_UP"
		}
		return "TRENDING_DOWN"
	}

	// 4. RANGING: теперь просто currentVol < meanVol
	if currentVol < meanVol {
		return "RANGING"
	}

	return "NEUTRAL"
}

// getVolatilityCategory определяет категорию волатильности
func getVolatilityCategory(volatility float64) string {
	switch {
	case volatility < 0.005:
		return "LOW"
	case volatility < 0.015:
		return "MEDIUM"
	default:
		return "HIGH"
	}
}

// analyzeEconomicIndicators анализирует экономические индикаторы
func analyzeEconomicIndicators(symbol string) map[string]float64 {
	indicators := make(map[string]float64)

	// Получение экономических данных
	data, err := fetchEconomicData(symbol)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching economic data")
		return indicators
	}

	// Анализ каждого индикатора
	for name, value := range data {
		// Нормализация значения
		normalizedValue := normalizeIndicator(value)

		// Применяем веса в зависимости от типа индикатора
		switch name {
		case "gdp_growth":
			indicators[name] = normalizedValue * 0.3
		case "inflation":
			indicators[name] = normalizedValue * 0.25
		case "interest_rate":
			indicators[name] = normalizedValue * 0.2
		case "unemployment":
			indicators[name] = normalizedValue * 0.15
		case "market_cap":
			indicators[name] = normalizedValue * 0.1
		default:
			indicators[name] = normalizedValue
		}
	}

	return indicators
}

// analyzeRiskFactors анализирует факторы риска
func analyzeRiskFactors(symbol string, candles []models.Candle) map[string]float64 {
	risks := make(map[string]float64)

	// Анализ волатильности
	volatilityRisk := calculateVolatilityRisk(candles)
	risks["volatility_risk"] = volatilityRisk

	// Анализ ликвидности
	liquidityRisk := calculateLiquidityRisk(candles)
	risks["liquidity_risk"] = liquidityRisk

	// Анализ корреляций
	correlationRisk := calculateCorrelationRisk(symbol)
	risks["correlation_risk"] = correlationRisk

	// Анализ новостей
	newsRisk := calculateNewsRisk(symbol)
	risks["news_risk"] = newsRisk

	// Анализ рыночных условий
	marketConditions := analyzeMarketConditions(candles)
	risks["market_risk"] = marketConditions["volatility"]*0.4 +
		marketConditions["spread"]*0.3 +
		marketConditions["market_depth"]*0.3

	// Общий риск
	totalRisk := 0.0
	for _, risk := range risks {
		totalRisk += risk
	}
	risks["total_risk"] = totalRisk / float64(len(risks))

	return risks
}

// AnalyzeMarket выполняет полный анализ рынка
func AnalyzeMarket(candles []models.Candle, marketCandles []models.Candle) *models.MarketAnalysis {
	analysis := &models.MarketAnalysis{
		Symbol:      candles[len(candles)-1].Symbol,
		TimeFrame:   candles[len(candles)-1].TimeFrame,
		LastPrice:   candles[len(candles)-1].Close,
		LastUpdated: time.Now(),
	}

	// Анализ настроений рынка
	sentiment := analyzeMarketSentiment(candles)
	analysis.MarketSentiment = sentiment

	// Анализ корреляций
	correlations := analyzeCorrelations(candles, marketCandles)
	analysis.Correlations = correlations

	// Анализ ликвидности
	lLiquidity := analyzeLiquidity(candles)
	analysis.Liquidity = lLiquidity

	// Анализ объемов
	volume := analyzeVolume(candles)
	analysis.Volume = volume

	// Анализ микроструктуры
	microstructure := analyzeMicrostructure(candles)
	analysis.Microstructure = microstructure

	// Анализ новостей
	news := analyzeNews(analysis.Symbol, analysis.TimeFrame)
	analysis.News = news

	// Фундаментальный анализ
	fundamentals := analyzeFundamentals(analysis.Symbol, candles)
	analysis.Fundamentals = fundamentals

	// Определение направления и уверенности
	analysis.Direction = determineDirection(sentiment, correlations, lLiquidity, volume, microstructure, news, fundamentals)
	analysis.Confidence = calculateConfidence(sentiment, correlations, lLiquidity, volume, microstructure, news, fundamentals)

	// Добавляем форматированный вывод режима и волатильности
	volatility := calculateVolatility(candles)
	analysis.MarketRegime = fmt.Sprintf("%s (%s)", determineMarketRegime(candles), getVolatilityCategory(volatility))
	analysis.RegimeStrength = calculateRegimeStrength(candles)
	analysis.Volatility = getVolatilityCategory(volatility)

	return analysis
}

// calculateMeanStd рассчитывает среднее значение и стандартное отклонение
func calculateMeanStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	// Расчет среднего
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	// Расчет стандартного отклонения
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	variance /= float64(len(values))
	std := math.Sqrt(variance)

	return mean, std
}
