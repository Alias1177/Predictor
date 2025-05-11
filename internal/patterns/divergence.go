package patterns

import (
	"math"

	"github.com/Alias1177/Predictor/models"
)

// DetectDivergences находит расхождения между ценой и индикаторами импульса
func DetectDivergences(candles []models.Candle, indicators *models.TechnicalIndicators) []models.Divergence {
	if len(candles) < 30 {
		return nil
	}

	var divergences []models.Divergence

	// Получаем исторические значения RSI
	rsiValues := make([]float64, len(candles))
	for i := 0; i < len(candles); i++ {
		windowSize := min(i+1, 14)
		if windowSize < 5 {
			rsiValues[i] = 50 // Значение по умолчанию при недостатке данных
			continue
		}

		// Используем значение из indicators для последней свечи, иначе вычисляем сами
		if i == len(candles)-1 && indicators != nil {
			rsiValues[i] = indicators.RSI
		} else {
			// Рассчитываем RSI самостоятельно без зависимости от calculate
			window := candles[max(0, i-windowSize+1) : i+1]
			rsiValues[i] = calculateRSI(window, windowSize)
		}
	}

	// Находим ценовые свинг-хаи и лоу
	swingHighs, swingLows := findSwingPoints(candles, 3)

	// Находим свинг-хаи и лоу RSI
	rsiSwingHighs, rsiSwingLows := findIndicatorSwings(rsiValues, 3)

	// Проверяем на обычные медвежьи дивергенции
	// Цена делает более высокий максимум, а RSI - более низкий максимум
	divergences = append(divergences, detectRegularBearishDivergence(candles, rsiValues, swingHighs, rsiSwingHighs)...)

	// Проверяем на обычные бычьи дивергенции
	// Цена делает более низкий минимум, а RSI - более высокий минимум
	divergences = append(divergences, detectRegularBullishDivergence(candles, rsiValues, swingLows, rsiSwingLows)...)

	// Проверяем на скрытые медвежьи дивергенции
	// Цена делает более низкий максимум, а RSI - более высокий максимум
	divergences = append(divergences, detectHiddenBearishDivergence(candles, rsiValues, swingHighs, rsiSwingHighs)...)

	// Проверяем на скрытые бычьи дивергенции
	// Цена делает более высокий минимум, а RSI - более низкий минимум
	divergences = append(divergences, detectHiddenBullishDivergence(candles, rsiValues, swingLows, rsiSwingLows)...)

	return divergences
}

// calculateRSI рассчитывает RSI напрямую, чтобы избежать зависимости от пакета calculate
func calculateRSI(candles []models.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50.0 // Значение по умолчанию при недостатке данных
	}

	var gains, losses float64
	// Рассчитываем начальные средние
	for i := 1; i <= period; i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	// Рассчитываем начальные средние
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Используем EMA (экспоненциальное скользящее среднее) для остальных данных
	for i := period + 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + 0) / float64(period)
		} else {
			avgGain = (avgGain*float64(period-1) + 0) / float64(period)
			avgLoss = (avgLoss*float64(period-1) - change) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

// findIndicatorSwings находит точки разворота в значениях индикатора
func findIndicatorSwings(values []float64, strength int) ([]int, []int) {
	var swingHighs, swingLows []int

	for i := strength; i < len(values)-strength; i++ {
		// Проверка на максимум
		isSwingHigh := true
		for j := i - strength; j < i; j++ {
			if values[j] > values[i] {
				isSwingHigh = false
				break
			}
		}
		for j := i + 1; j <= i+strength; j++ {
			if values[j] > values[i] {
				isSwingHigh = false
				break
			}
		}

		if isSwingHigh {
			swingHighs = append(swingHighs, i)
		}

		// Проверка на минимум
		isSwingLow := true
		for j := i - strength; j < i; j++ {
			if values[j] < values[i] {
				isSwingLow = false
				break
			}
		}
		for j := i + 1; j <= i+strength; j++ {
			if values[j] < values[i] {
				isSwingLow = false
				break
			}
		}

		if isSwingLow {
			swingLows = append(swingLows, i)
		}
	}

	return swingHighs, swingLows
}

func detectRegularBearishDivergence(candles []models.Candle, rsiValues []float64, priceSwingHighs, rsiSwingHighs []int) []models.Divergence {
	var divergences []models.Divergence

	// Необходимо как минимум 2 свинг-хая для сравнения
	if len(priceSwingHighs) < 2 || len(rsiSwingHighs) < 2 {
		return divergences
	}

	// Рассматриваем последние 2 ценовых свинг-хая
	for i := len(priceSwingHighs) - 1; i > 0; i-- {
		p2 := priceSwingHighs[i]
		p1 := priceSwingHighs[i-1]

		// Проверяем, сделала ли цена более высокий максимум
		if candles[p2].High <= candles[p1].High {
			continue
		}

		// Находим ближайшие свинг-хаи RSI
		r1, r2 := findClosestSwings(p1, p2, rsiSwingHighs)
		if r1 < 0 || r2 < 0 {
			continue
		}

		// Проверяем, сделал ли RSI более низкий максимум
		if rsiValues[r2] >= rsiValues[r1] {
			continue
		}

		// Мы нашли обычную медвежью дивергенцию
		divergence := models.Divergence{
			Type:      "REGULAR",
			Direction: "BEARISH",
			PricePoints: []models.DivergencePoint{
				{Index: p1, Value: candles[p1].High},
				{Index: p2, Value: candles[p2].High},
			},
			IndicatorPoints: []models.DivergencePoint{
				{Index: r1, Value: rsiValues[r1]},
				{Index: r2, Value: rsiValues[r2]},
			},
			Indicator: "RSI",
			SignalStrength: calculateDivergenceStrength(
				candles[p2].High/candles[p1].High,
				rsiValues[r1]/rsiValues[r2],
			),
		}

		divergences = append(divergences, divergence)
	}

	return divergences
}

// Реализации других функций обнаружения дивергенций
func detectRegularBullishDivergence(candles []models.Candle, rsiValues []float64, priceSwingLows, rsiSwingLows []int) []models.Divergence {
	var divergences []models.Divergence

	// Необходимо как минимум 2 свинг-лоу для сравнения
	if len(priceSwingLows) < 2 || len(rsiSwingLows) < 2 {
		return divergences
	}

	// Рассматриваем последние 2 ценовых свинг-лоу
	for i := len(priceSwingLows) - 1; i > 0; i-- {
		p2 := priceSwingLows[i]
		p1 := priceSwingLows[i-1]

		// Проверяем, сделала ли цена более низкий минимум
		if candles[p2].Low >= candles[p1].Low {
			continue
		}

		// Находим ближайшие свинг-лоу RSI
		r1, r2 := findClosestSwings(p1, p2, rsiSwingLows)
		if r1 < 0 || r2 < 0 {
			continue
		}

		// Проверяем, сделал ли RSI более высокий минимум
		if rsiValues[r2] <= rsiValues[r1] {
			continue
		}

		// Мы нашли обычную бычью дивергенцию
		divergence := models.Divergence{
			Type:      "REGULAR",
			Direction: "BULLISH",
			PricePoints: []models.DivergencePoint{
				{Index: p1, Value: candles[p1].Low},
				{Index: p2, Value: candles[p2].Low},
			},
			IndicatorPoints: []models.DivergencePoint{
				{Index: r1, Value: rsiValues[r1]},
				{Index: r2, Value: rsiValues[r2]},
			},
			Indicator: "RSI",
			SignalStrength: calculateDivergenceStrength(
				candles[p1].Low/candles[p2].Low, // Инвертируем соотношение
				rsiValues[r2]/rsiValues[r1],
			),
		}

		divergences = append(divergences, divergence)
	}

	return divergences
}

func detectHiddenBearishDivergence(candles []models.Candle, rsiValues []float64, priceSwingHighs, rsiSwingHighs []int) []models.Divergence {
	var divergences []models.Divergence

	// Необходимо как минимум 2 свинг-хая для сравнения
	if len(priceSwingHighs) < 2 || len(rsiSwingHighs) < 2 {
		return divergences
	}

	// Рассматриваем последние 2 ценовых свинг-хая
	for i := len(priceSwingHighs) - 1; i > 0; i-- {
		p2 := priceSwingHighs[i]
		p1 := priceSwingHighs[i-1]

		// Проверяем, сделала ли цена более низкий максимум
		if candles[p2].High >= candles[p1].High {
			continue
		}

		// Находим ближайшие свинг-хаи RSI
		r1, r2 := findClosestSwings(p1, p2, rsiSwingHighs)
		if r1 < 0 || r2 < 0 {
			continue
		}

		// Проверяем, сделал ли RSI более высокий максимум
		if rsiValues[r2] <= rsiValues[r1] {
			continue
		}

		// Мы нашли скрытую медвежью дивергенцию
		divergence := models.Divergence{
			Type:      "HIDDEN",
			Direction: "BEARISH",
			PricePoints: []models.DivergencePoint{
				{Index: p1, Value: candles[p1].High},
				{Index: p2, Value: candles[p2].High},
			},
			IndicatorPoints: []models.DivergencePoint{
				{Index: r1, Value: rsiValues[r1]},
				{Index: r2, Value: rsiValues[r2]},
			},
			Indicator: "RSI",
			SignalStrength: calculateDivergenceStrength(
				candles[p1].High/candles[p2].High,
				rsiValues[r2]/rsiValues[r1],
			),
		}

		divergences = append(divergences, divergence)
	}

	return divergences
}

func detectHiddenBullishDivergence(candles []models.Candle, rsiValues []float64, priceSwingLows, rsiSwingLows []int) []models.Divergence {
	var divergences []models.Divergence

	// Необходимо как минимум 2 свинг-лоу для сравнения
	if len(priceSwingLows) < 2 || len(rsiSwingLows) < 2 {
		return divergences
	}

	// Рассматриваем последние 2 ценовых свинг-лоу
	for i := len(priceSwingLows) - 1; i > 0; i-- {
		p2 := priceSwingLows[i]
		p1 := priceSwingLows[i-1]

		// Проверяем, сделала ли цена более высокий минимум
		if candles[p2].Low <= candles[p1].Low {
			continue
		}

		// Находим ближайшие свинг-лоу RSI
		r1, r2 := findClosestSwings(p1, p2, rsiSwingLows)
		if r1 < 0 || r2 < 0 {
			continue
		}

		// Проверяем, сделал ли RSI более низкий минимум
		if rsiValues[r2] >= rsiValues[r1] {
			continue
		}

		// Мы нашли скрытую бычью дивергенцию
		divergence := models.Divergence{
			Type:      "HIDDEN",
			Direction: "BULLISH",
			PricePoints: []models.DivergencePoint{
				{Index: p1, Value: candles[p1].Low},
				{Index: p2, Value: candles[p2].Low},
			},
			IndicatorPoints: []models.DivergencePoint{
				{Index: r1, Value: rsiValues[r1]},
				{Index: r2, Value: rsiValues[r2]},
			},
			Indicator: "RSI",
			SignalStrength: calculateDivergenceStrength(
				candles[p2].Low/candles[p1].Low,
				rsiValues[r1]/rsiValues[r2],
			),
		}

		divergences = append(divergences, divergence)
	}

	return divergences
}

func findClosestSwings(p1, p2 int, swings []int) (int, int) {
	var closest1, closest2 = -1, -1
	minDist1, minDist2 := 1000, 1000

	for _, s := range swings {
		dist1 := abs(s - p1)
		if dist1 < minDist1 {
			minDist1 = dist1
			closest1 = s
		}

		dist2 := abs(s - p2)
		if dist2 < minDist2 {
			minDist2 = dist2
			closest2 = s
		}
	}

	// Возвращаем действительные пары только в правильном порядке
	if closest1 < closest2 {
		return closest1, closest2
	}

	return -1, -1
}

func calculateDivergenceStrength(priceRatio, indicatorRatio float64) float64 {
	// Простой расчет силы на основе разницы между соотношениями
	if indicatorRatio == 0 {
		return 0
	}
	return math.Abs(priceRatio - indicatorRatio)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
