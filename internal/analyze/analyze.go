package analyze

import (
	"math"

	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
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
