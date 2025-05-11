package calculate

import (
	"math"

	"github.com/Alias1177/Predictor/models"
)

// determineStopLoss определяет уровень стоп-лосса на основе нескольких методов
func DetermineStopLoss(candles []models.Candle, indicators *models.TechnicalIndicators, direction string) float64 {
	currentPrice := candles[len(candles)-1].Close

	// Метод 1: ATR-базированный стоп
	atrMultiplier := 0.0

	// Динамический мультипликатор ATR, зависящий от волатильности
	if indicators.VolatilityRatio > 1.5 {
		// Высокая волатильность - более широкий стоп
		atrMultiplier = 2.5
	} else if indicators.VolatilityRatio < 0.7 {
		// Низкая волатильность - более узкий стоп
		atrMultiplier = 1.2
	} else {
		// Нормальная волатильность
		atrMultiplier = 1.8
	}

	atrStop := 0.0
	if direction == "UP" {
		atrStop = currentPrice - (indicators.ATR * atrMultiplier)
	} else {
		atrStop = currentPrice + (indicators.ATR * atrMultiplier)
	}

	// Метод 2: Стоп на основе уровней поддержки/сопротивления
	structureStop := 0.0
	if direction == "UP" {
		// Для длинной позиции ищем ближайший уровень поддержки
		supportLevel := findNearestSupportLevel(currentPrice, indicators.Support)
		structureStop = supportLevel * 0.998 // Чуть ниже уровня поддержки
	} else {
		// Для короткой позиции ищем ближайший уровень сопротивления
		resistanceLevel := findNearestResistanceLevel(currentPrice, indicators.Resistance)
		structureStop = resistanceLevel * 1.002 // Чуть выше уровня сопротивления
	}

	// Метод 3: Процентный стоп (запасной вариант)
	percentStop := 0.0
	if direction == "UP" {
		percentStop = currentPrice * 0.99 // 1% стоп-лосс для длинной позиции
	} else {
		percentStop = currentPrice * 1.01 // 1% стоп-лосс для короткой позиции
	}

	// Выбираем наиболее консервативный стоп
	var stop float64
	if direction == "UP" {
		// Для длинной позиции выбираем наибольший (самый близкий к цене) стоп
		stop = math.Max(atrStop, structureStop)
		stop = math.Max(stop, percentStop)
	} else {
		// Для короткой позиции выбираем наименьший (самый близкий к цене) стоп
		stop = math.Min(atrStop, structureStop)
		stop = math.Min(stop, percentStop)
	}

	return stop
}

// findNearestSupportLevel находит ближайший уровень поддержки ниже текущей цены
func findNearestSupportLevel(currentPrice float64, supportLevels []float64) float64 {
	if len(supportLevels) == 0 {
		return currentPrice * 0.97 // Если уровней нет, используем -3% от текущей цены
	}

	nearest := 0.0
	minDistance := math.MaxFloat64

	for _, level := range supportLevels {
		if level < currentPrice {
			distance := currentPrice - level
			if distance < minDistance {
				minDistance = distance
				nearest = level
			}
		}
	}

	if nearest == 0.0 {
		// Если не найдено уровней поддержки ниже цены, используем -3%
		return currentPrice * 0.97
	}

	return nearest
}

// findNearestResistanceLevel находит ближайший уровень сопротивления выше текущей цены
func findNearestResistanceLevel(currentPrice float64, resistanceLevels []float64) float64 {
	if len(resistanceLevels) == 0 {
		return currentPrice * 1.03 // Если уровней нет, используем +3% от текущей цены
	}

	nearest := 0.0
	minDistance := math.MaxFloat64

	for _, level := range resistanceLevels {
		if level > currentPrice {
			distance := level - currentPrice
			if distance < minDistance {
				minDistance = distance
				nearest = level
			}
		}
	}

	if nearest == 0.0 {
		// Если не найдено уровней сопротивления выше цены, используем +3%
		return currentPrice * 1.03
	}

	return nearest
}

// Функция для расчета размера позиции
// calculatePositionSize вычисляет оптимальный размер позиции с учетом риск-менеджмента
func CalculatePositionSize(currentPrice float64, stopLoss float64, accountSize float64, riskPerTrade float64) *models.PositionSizingResult {
	// Проверка входных данных
	if currentPrice <= 0 || accountSize <= 0 || riskPerTrade <= 0 {
		return &models.PositionSizingResult{
			PositionSize:    0,
			StopLoss:        stopLoss,
			TakeProfit:      currentPrice,
			RiskRewardRatio: 0,
			AccountRisk:     riskPerTrade,
		}
	}

	// Вычисляем размер стопа в пунктах
	stopSizePoints := math.Abs(currentPrice - stopLoss)

	if stopSizePoints <= 0 {
		// Защита от деления на ноль
		stopSizePoints = currentPrice * 0.01 // 1% от цены
	}

	// Вычисляем риск в деньгах
	riskAmount := accountSize * riskPerTrade

	// Размер позиции = Риск в деньгах / Размер стопа в деньгах
	positionSize := riskAmount / stopSizePoints

	// Определяем разные уровни тейк-профитов
	takeProfitRatio1 := 1.5 // R:R 1:1.5 (консервативная цель)
	takeProfitRatio2 := 2.0 // R:R 1:2 (стандартная цель)
	takeProfitRatio3 := 3.0 // R:R 1:3 (агрессивная цель)

	takeProfit1 := 0.0
	takeProfit2 := 0.0
	takeProfit3 := 0.0

	if currentPrice > stopLoss {
		// Long позиция
		takeProfit1 = currentPrice + (currentPrice-stopLoss)*takeProfitRatio1
		takeProfit2 = currentPrice + (currentPrice-stopLoss)*takeProfitRatio2
		takeProfit3 = currentPrice + (currentPrice-stopLoss)*takeProfitRatio3
	} else {
		// Short позиция
		takeProfit1 = currentPrice - (stopLoss-currentPrice)*takeProfitRatio1
		takeProfit2 = currentPrice - (stopLoss-currentPrice)*takeProfitRatio2
		takeProfit3 = currentPrice - (stopLoss-currentPrice)*takeProfitRatio3
	}

	// По умолчанию используем стандартную цель (1:2)
	selectedTakeProfit := takeProfit2
	selectedRiskRewardRatio := takeProfitRatio2

	return &models.PositionSizingResult{
		PositionSize:    positionSize,
		StopLoss:        stopLoss,
		TakeProfit:      selectedTakeProfit,
		RiskRewardRatio: selectedRiskRewardRatio,
		AccountRisk:     riskPerTrade,
		AdditionalTargets: map[string]float64{
			"conservative": takeProfit1,
			"standard":     takeProfit2,
			"aggressive":   takeProfit3,
		},
	}
}
