package calculate

import (
	"github.com/Alias1177/Predictor/models"
	"math"
)

func determineStopLoss(candles []models.Candle, indicators *models.TechnicalIndicators, direction string) float64 {
	currentPrice := candles[len(candles)-1].Close
	atr := indicators.ATR

	// Базовый расчет стопа на основе ATR
	atrMultiplier := 1.5

	defaultStop := 0.0
	if direction == "UP" {
		defaultStop = currentPrice - (atr * atrMultiplier)
	} else {
		defaultStop = currentPrice + (atr * atrMultiplier)
	}

	return defaultStop
}

// Функция для расчета размера позиции
func calculatePositionSize(currentPrice float64, stopLoss float64, accountSize float64, riskPerTrade float64) *models.PositionSizingResult {
	// Вычисляем размер стопа в пунктах
	stopSizePoints := math.Abs(currentPrice - stopLoss)

	// Вычисляем риск в деньгах
	riskAmount := accountSize * riskPerTrade

	// Размер позиции = Риск в деньгах / Размер стопа в деньгах
	positionSize := riskAmount / stopSizePoints

	// Определяем уровень тейк-профита с соотношением 1:2
	takeProfit := 0.0
	if currentPrice > stopLoss {
		// Long позиция
		takeProfit = currentPrice + (currentPrice-stopLoss)*2.0
	} else {
		// Short позиция
		takeProfit = currentPrice - (stopLoss-currentPrice)*2.0
	}

	// Рассчитываем соотношение риск/прибыль
	riskRewardRatio := 0.0
	if stopSizePoints > 0 {
		riskRewardRatio = math.Abs(takeProfit-currentPrice) / stopSizePoints
	}

	return &models.PositionSizingResult{
		PositionSize:    positionSize,
		StopLoss:        stopLoss,
		TakeProfit:      takeProfit,
		RiskRewardRatio: riskRewardRatio,
		AccountRisk:     riskPerTrade,
	}
}
