package baktest

import (
	"chi/Predictor/config"

	"chi/Predictor/models"
	"context"
	"fmt"
	"math"
	"time"
)

func RunBacktest(ctx context.Context, client *config.Client, config *models.Config) (*models.BacktestResults, error) {
	// Загружаем исторические свечи
	historicalCandles, err := client.GetHistoricalCandles(ctx, config.BacktestDays)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical data: %w", err)
	}

	if len(historicalCandles) < 100 {
		return nil, fmt.Errorf("insufficient historical data for backtesting, got %d candles", len(historicalCandles))
	}

	// Инициализируем результаты
	results := &models.BacktestResults{
		MaxConsecutive: struct {
			Wins  int `json:"wins"`
			Loses int `json:"loses"`
		}{},
		MarketRegimePerformance: make(map[string]float64),
		TimeframePerformance:    make(map[string]float64),
		DetailedResults:         []models.PredictionResult{},
	}

	// Отслеживаем прибыль и убытки
	var totalProfit, totalLoss float64

	// Устанавливаем размер окна для проверки
	windowSize := config.CandleCount
	predictionInterval := 1 // Сколько свечей вперёд проверяем

	// Устанавливаем лимит для валидации
	validationLimit := len(historicalCandles) - predictionInterval
	consecutiveWins := 0
	consecutiveLosses := 0

	// Статистика по рыночным режимам
	regimeStats := map[string]struct {
		correct int
		total   int
	}{
		"TRENDING": {0, 0},
		"RANGING":  {0, 0},
		"CHOPPY":   {0, 0},
		"VOLATILE": {0, 0},
		"UNKNOWN":  {0, 0},
	}

	// Виртуальный баланс счета для отслеживания эквити
	accountBalance := 10000.0 // Начальный баланс
	balanceHistory := []float64{accountBalance}
	highWaterMark := accountBalance
	maxDrawdown := 0.0

	// Для каждой позиции в окне
	for i := windowSize; i < validationLimit; i += predictionInterval {
		// Извлекаем тестовое окно
		testWindow := historicalCandles[i-windowSize : i]

		// Рассчитываем индикаторы для этого окна
		indicators := calculateAllIndicators(testWindow, config)

		// Получаем рыночный режим и аномалии
		regime := enhancedMarketRegimeClassification(testWindow)
		anomaly := detectMarketAnomalies(testWindow)

		// Получаем мультитаймфреймовые данные
		mtfData := map[string][]Candle{
			"5min": testWindow,
		}

		// Генерируем прогноз
		direction, confidence, score, factors := enhancedPrediction(
			testWindow, indicators, mtfData, regime, anomaly, config)

		// Создаем запись о прогнозе
		result := PredictionResult{
			Direction:        direction,
			Confidence:       confidence,
			Score:            score,
			Factors:          factors,
			Timestamp:        time.Now().Add(-time.Duration(validationLimit-i) * time.Minute * 5),
			PredictionID:     fmt.Sprintf("BT-%d", i),
			PredictionTarget: time.Now().Add(-time.Duration(validationLimit-i-predictionInterval) * time.Minute * 5),
		}

		// Фильтрация сигналов (только высокая уверенность или сильный сигнал)
		if direction == "NEUTRAL" || (confidence != "HIGH" && math.Abs(score) < 0.3) {
			continue // Пропускаем сделки с низкой уверенностью
		}

		// Фильтр режима рынка
		if regime.Type == "CHOPPY" && regime.Strength > 0.7 {
			continue // Пропускаем высоко хаотичные рынки
		}

		// Определяем будущую цену для проверки
		currentPrice := testWindow[len(testWindow)-1].Close
		futurePrice := historicalCandles[i+predictionInterval].Close
		priceChange := futurePrice - currentPrice

		// Определяем фактический результат
		actualOutcome := "NEUTRAL"
		if priceChange > 0 {
			actualOutcome = "UP"
		} else if priceChange < 0 {
			actualOutcome = "DOWN"
		}

		result.ActualOutcome = actualOutcome

		// Проверяем, был ли прогноз верным
		wasCorrect := direction == actualOutcome
		result.WasCorrect = wasCorrect

		// Добавляем результат в список
		results.DetailedResults = append(results.DetailedResults, result)
		results.TotalTrades++

		// Обновляем счетчики побед/поражений и баланс счета
		if wasCorrect {
			results.WinningTrades++
			consecutiveWins++
			consecutiveLosses = 0

			// Расчет прибыли
			profit := math.Abs(priceChange) * 100 // Примерная прибыль в пунктах
			totalProfit += profit

			// Обновляем баланс счета
			accountBalance += profit
		} else {
			results.LosingTrades++
			consecutiveLosses++
			consecutiveWins = 0

			// Расчет убытка
			loss := math.Abs(priceChange) * 100 // Примерная потеря в пунктах
			totalLoss += loss

			// Обновляем баланс счета
			accountBalance -= loss
		}

		// Отслеживаем баланс и просадку
		balanceHistory = append(balanceHistory, accountBalance)
		if accountBalance > highWaterMark {
			highWaterMark = accountBalance
		} else {
			currentDrawdown := (highWaterMark - accountBalance) / highWaterMark
			if currentDrawdown > maxDrawdown {
				maxDrawdown = currentDrawdown
			}
		}

		// Обновляем максимальные последовательные значения
		if consecutiveWins > results.MaxConsecutive.Wins {
			results.MaxConsecutive.Wins = consecutiveWins
		}
		if consecutiveLosses > results.MaxConsecutive.Loses {
			results.MaxConsecutive.Loses = consecutiveLosses
		}

		// Обновляем статистику по рыночным режимам
		if stats, exists := regimeStats[regime.Type]; exists {
			stats.total++
			if wasCorrect {
				stats.correct++
			}
			regimeStats[regime.Type] = stats
		}
	}

	// Рассчитываем процентные метрики
	if results.TotalTrades > 0 {
		results.WinPercentage = float64(results.WinningTrades) / float64(results.TotalTrades) * 100
	}

	// Средний выигрыш и проигрыш
	if results.WinningTrades > 0 {
		results.AverageGain = totalProfit / float64(results.WinningTrades)
	}
	if results.LosingTrades > 0 {
		results.AverageLoss = totalLoss / float64(results.LosingTrades)
	}

	// Нормализуем статистику рыночных режимов в проценты
	for regime, stats := range regimeStats {
		if stats.total > 0 {
			results.MarketRegimePerformance[regime] = float64(stats.correct) / float64(stats.total) * 100
		}
	}
	if len(balanceHistory) > 0 {
		initialBalance := balanceHistory[0]
		finalBalance := balanceHistory[len(balanceHistory)-1]
		results.EquityGrowthPercent = ((finalBalance - initialBalance) / initialBalance) * 100
	}
	monthlyReturns := make(map[string]float64)
	if len(results.DetailedResults) > 0 {
		for _, result := range results.DetailedResults {
			month := result.Timestamp.Format("2006-01")
			if result.WasCorrect {
				monthlyReturns[month] += results.AverageGain
			} else {
				monthlyReturns[month] -= results.AverageLoss
			}
		}

		// Преобразуем абсолютные значения в проценты
		initialBalance := 10000.0
		for month, value := range monthlyReturns {
			monthlyReturns[month] = (value / initialBalance) * 100
		}

		results.MonthlyReturns = monthlyReturns
	}
	if results.TotalTrades > 0 {
		results.WinPercentage = float64(results.WinningTrades) / float64(results.TotalTrades) * 100
	}

	// Средний выигрыш и проигрыш
	if results.WinningTrades > 0 {
		results.AverageGain = totalProfit / float64(results.WinningTrades)
	}
	if results.LosingTrades > 0 {
		results.AverageLoss = totalLoss / float64(results.LosingTrades)
	}

	// Расчет коэффициента прибыли
	if totalLoss > 0 {
		results.ProfitFactor = totalProfit / totalLoss
	} else {
		results.ProfitFactor = totalProfit // Если нет убытков
	}

	// Сохраняем максимальную просадку
	results.MaxDrawdown = maxDrawdown * 100 // В процентах

	// Нормализуем статистику рыночных режимов в проценты
	for regime, stats := range regimeStats {
		if stats.total > 0 {
			results.MarketRegimePerformance[regime] = float64(stats.correct) / float64(stats.total) * 100
		}
	}

	// Рост капитала в процентах
	if len(balanceHistory) > 0 {
		initialBalance := balanceHistory[0]
		finalBalance := balanceHistory[len(balanceHistory)-1]
		results.EquityGrowthPercent = ((finalBalance - initialBalance) / initialBalance) * 100
	}

	// Расчет месячной доходности
	results.MonthlyReturns = make(map[string]float64)
	if len(results.DetailedResults) > 0 {
		for _, result := range results.DetailedResults {
			month := result.Timestamp.Format("2006-01")
			if result.WasCorrect {
				results.MonthlyReturns[month] += results.AverageGain
			} else {
				results.MonthlyReturns[month] -= results.AverageLoss
			}
		}

		// Преобразуем абсолютные значения в проценты
		initialBalance := 10000.0
		for month, value := range results.MonthlyReturns {
			results.MonthlyReturns[month] = (value / initialBalance) * 100
		}
	}

	// Расчет процентных показателей относительно цены
	basePrice := 0.0
	if len(historicalCandles) > 0 {
		for i := len(historicalCandles) - minInt(20, len(historicalCandles)); i < len(historicalCandles); i++ {
			basePrice += historicalCandles[i].Close
		}
		basePrice /= float64(minInt(20, len(historicalCandles)))
	}

	if basePrice > 0 {
		// Преобразуем абсолютные значения пунктов в проценты
		results.AverageGainPercent = (results.AverageGain / basePrice) * 100
		results.AverageLossPercent = (results.AverageLoss / basePrice) * 100
	}

	// Рассчитываем общий процент прибыли/убытка
	initialBalance := 10000.0
	finalBalance := initialBalance + (float64(results.WinningTrades) * results.AverageGain) -
		(float64(results.LosingTrades) * results.AverageLoss)
	results.TotalReturnPercent = ((finalBalance - initialBalance) / initialBalance) * 100

	return results, nil
}
