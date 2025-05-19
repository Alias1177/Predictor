package anomaly

import (
	"fmt"
	"math"

	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
)

// MarketStateHMM реализует упрощенную скрытую марковскую модель для определения режима рынка
func MarketStateHMM(candles []models.Candle, windowSize int) *models.MarketRegime {
	if len(candles) < windowSize*2 {
		return &models.MarketRegime{
			Type:             "UNKNOWN",
			Strength:         0,
			Direction:        "NEUTRAL",
			VolatilityLevel:  "NORMAL",
			MomentumStrength: 0,
			LiquidityRating:  "NORMAL",
			PriceStructure:   "UNKNOWN",
		}
	}

	// Рассчитываем последовательности доходности
	returns := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		returns[i-1] = (candles[i].Close - candles[i-1].Close) / candles[i-1].Close
	}

	// Рассчитываем волатильность в скользящих окнах
	volatilities := make([]float64, len(returns)-windowSize+1)
	for i := 0; i <= len(returns)-windowSize; i++ {
		windowReturns := returns[i : i+windowSize]
		volatilities[i] = calculateReturnsVolatility(windowReturns)
	}

	// Рассчитываем среднее и стандартное отклонение волатильности для определения режимов
	meanVol, stdVol := calculateMeanStd(volatilities)

	// Текущая волатильность (последнее окно)
	currentVol := volatilities[len(volatilities)-1]

	// Определяем состояние рынка
	regime := &models.MarketRegime{
		LiquidityRating: "NORMAL",
		PriceStructure:  "UNKNOWN",
	}

	if currentVol > meanVol+stdVol*1.5 {
		regime.Type = "VOLATILE"
		regime.VolatilityLevel = "HIGH"
		regime.Strength = math.Min((currentVol-meanVol)/stdVol/3, 1.0)
	} else if currentVol < meanVol-stdVol*0.5 {
		// Низкая волатильность может указывать на флэт или предпробойное состояние
		regime.Type = "RANGING"
		regime.VolatilityLevel = "LOW"
		regime.Strength = math.Min((meanVol-currentVol)/stdVol, 1.0)
	} else {
		// Нормальная волатильность часто указывает на тренд
		adx, plusDI, minusDI := utils.CalculateADX(candles, windowSize)
		if adx > 25 {
			regime.Type = "TRENDING"
			regime.Strength = math.Min(adx/50.0, 1.0)
			if plusDI > minusDI {
				regime.Direction = "BULLISH"
				regime.PriceStructure = "TRENDING_UP"
			} else {
				regime.Direction = "BEARISH"
				regime.PriceStructure = "TRENDING_DOWN"
			}
		} else {
			regime.Type = "CHOPPY"
			regime.Strength = 0.5
			regime.Direction = "NEUTRAL"
		}
	}

	// Определяем силу импульса на основе недавнего движения цены
	currentPrice := candles[len(candles)-1].Close
	prevPrice := candles[len(candles)-windowSize].Close
	momentumPct := (currentPrice - prevPrice) / prevPrice
	regime.MomentumStrength = math.Min(math.Abs(momentumPct)*50, 1.0)

	if momentumPct > 0 {
		if regime.Direction == "NEUTRAL" {
			regime.Direction = "BULLISH"
		}
	} else if momentumPct < 0 {
		if regime.Direction == "NEUTRAL" {
			regime.Direction = "BEARISH"
		}
	}

	// Анализ ликвидности на основе ATR относительно цены
	avgPrice := (currentPrice + prevPrice) / 2
	atr10 := utils.CalculateATR(candles, 10)
	liquidityRatio := atr10 / avgPrice

	if liquidityRatio > 0.005 { // 0.5% волатильность относительно цены
		regime.LiquidityRating = "LOW"
	} else if liquidityRatio < 0.001 { // 0.1% волатильность относительно цены
		regime.LiquidityRating = "HIGH"
	}

	return regime
}

// Удаляем старые функции calculateMarketFeatures и calculateVolumeChange

// Улучшенная функция определения режима
func EnhancedMarketRegimeClassification(candles []models.Candle) (*models.MarketRegime, error) {
	if len(candles) < 50 {
		return &models.MarketRegime{
			Type:             "UNKNOWN",
			Strength:         0,
			Direction:        "NEUTRAL",
			VolatilityLevel:  "NORMAL",
			MomentumStrength: 0,
			LiquidityRating:  "NORMAL",
			PriceStructure:   "UNKNOWN",
		}, nil
	}

	features, err := utils.CalculateMarketFeatures(candles)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate market features: %w", err)
	}

	// Определяем режим на основе признаков
	regime := &models.MarketRegime{
		Type:             "UNKNOWN",
		Direction:        "NEUTRAL",
		LiquidityRating:  "NORMAL",
		PriceStructure:   "UNKNOWN",
		MomentumStrength: 0.5,
	}

	// Анализ волатильности
	volatility := features[0]
	// Адаптивные пороги на основе исторических данных
	historicalVolatilities := make([]float64, 0)
	for i := 0; i < len(candles)-1; i++ {
		historicalVolatilities = append(historicalVolatilities, math.Abs((candles[i+1].Close-candles[i].Close)/candles[i].Close))
	}
	meanVol, stdVol := calculateMeanStd(historicalVolatilities)

	if volatility > meanVol+stdVol*1.2 {
		regime.Type = "VOLATILE"
		regime.VolatilityLevel = "HIGH"
		regime.Strength = math.Min((volatility-meanVol)/stdVol/2, 1.0)
	} else if volatility < meanVol-stdVol*0.8 {
		regime.Type = "RANGING"
		regime.VolatilityLevel = "LOW"
		priceStability := 1.0 - (volatility / (meanVol - stdVol*0.8))
		regime.Strength = math.Min(priceStability*1.2, 1.0)
	} else {
		regime.Type = "TRENDING"
		regime.VolatilityLevel = "NORMAL"
		regime.Strength = 0.5
	}

	// Анализ тренда с адаптивными порогами
	trend := features[1]
	historicalTrends := make([]float64, 0)
	for i := 20; i < len(candles); i++ {
		historicalTrends = append(historicalTrends, (candles[i].Close-candles[i-20].Close)/candles[i-20].Close)
	}
	meanTrend, stdTrend := calculateMeanStd(historicalTrends)

	if math.Abs(trend) > meanTrend+stdTrend*0.8 {
		if trend > 0 {
			regime.Direction = "BULLISH"
			regime.PriceStructure = "TRENDING_UP"
		} else {
			regime.Direction = "BEARISH"
			regime.PriceStructure = "TRENDING_DOWN"
		}
		regime.Strength = math.Min(math.Abs(trend)/(meanTrend+stdTrend*0.8), 1.0)
	} else {
		regime.Direction = "NEUTRAL"
		regime.Strength = math.Max(regime.Strength, 0.3)
	}

	// Анализ моментума
	momenta := features[2]
	if momenta > 70 {
		regime.MomentumStrength = 0.8
		if regime.Direction == "NEUTRAL" {
			regime.Direction = "BULLISH"
		}
	} else if momenta < 30 {
		regime.MomentumStrength = 0.2
		if regime.Direction == "NEUTRAL" {
			regime.Direction = "BEARISH"
		}
	} else {
		regime.MomentumStrength = 0.5
	}

	// Анализ ликвидности
	volumeChange := features[3]
	if volumeChange > 0.5 {
		regime.LiquidityRating = "HIGH"
	} else if volumeChange < -0.5 {
		regime.LiquidityRating = "LOW"
	}

	return regime, nil
}

func calculateReturnsVolatility(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}

	if len(returns) <= 1 {
		return 0
	}

	variance /= float64(len(returns) - 1)
	return math.Sqrt(variance)
}

func calculateMeanStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	if len(values) <= 1 {
		return mean, 0 // Невозможно вычислить std для одного значения
	}

	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	// Используем n-1 для несмещенной оценки
	variance /= float64(len(values) - 1)

	return mean, math.Sqrt(variance)
}
