package calculate

import (
	"math"

	"github.com/Alias1177/Predictor/internal/anomaly"
	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
)

// adaptIndicatorParameters динамически корректирует параметры индикаторов на основе рыночных условий
func AdaptIndicatorParameters(candles []models.Candle, config *models.Config) *models.Config {
	if !config.AdaptiveIndicator || len(candles) < 30 {
		return config // Возвращаем исходный конфиг, если адаптация отключена или недостаточно данных
	}

	// Создаем копию конфига
	adaptedConfig := *config

	// Расчет показателей волатильности
	atr5 := utils.CalculateATR(candles, 5)
	atr20 := utils.CalculateATR(candles, 20)
	volatilityRatio := 1.0
	if atr20 > 0 {
		volatilityRatio = atr5 / atr20
	}

	// Проверка рыночного режима
	regime := anomaly.EnhancedMarketRegimeClassification(candles)

	// Корректировка периода RSI на основе волатильности
	if volatilityRatio > 1.5 {
		// Высокая волатильность - используем более короткие периоды для быстрой реакции
		adaptedConfig.RSIPeriod = utils.MaxInt(5, config.RSIPeriod-2)
	} else if volatilityRatio < 0.7 {
		// Низкая волатильность - используем более длинные периоды для уменьшения шума
		adaptedConfig.RSIPeriod = utils.MinInt(14, config.RSIPeriod+2)
	}

	// Корректировка параметров полос Боллинджера на основе рыночного режима
	if regime.Type == "TRENDING" && regime.Strength > 0.7 {
		// В сильных трендах расширяем полосы
		adaptedConfig.BBStdDev = math.Min(3.0, config.BBStdDev+0.3)
	} else if regime.Type == "RANGING" {
		// В флэтовых рынках сужаем полосы
		adaptedConfig.BBStdDev = math.Max(1.8, config.BBStdDev-0.3)
	}

	// Корректировка параметров MACD на основе рыночного режима
	if regime.Type == "CHOPPY" || regime.Type == "RANGING" {
		// В нетрендовых рынках используем более широкие настройки MACD
		adaptedConfig.MACDFastPeriod = utils.MinInt(12, config.MACDFastPeriod+2)
		adaptedConfig.MACDSlowPeriod = utils.MinInt(26, config.MACDSlowPeriod+3)
	} else if regime.Type == "TRENDING" && regime.Strength > 0.6 {
		// В трендовых рынках используем более чувствительные настройки MACD
		adaptedConfig.MACDFastPeriod = utils.MaxInt(5, config.MACDFastPeriod-1)
		adaptedConfig.MACDSlowPeriod = utils.MaxInt(12, config.MACDSlowPeriod-2)
	}

	// Корректировка периода EMA на основе импульса
	if regime.MomentumStrength > 0.7 {
		// Сильный импульс - используем более короткий период EMA
		adaptedConfig.EMAPeriod = utils.MaxInt(8, config.EMAPeriod-2)
	} else if regime.MomentumStrength < 0.3 {
		// Слабый импульс - используем более длинный период EMA
		adaptedConfig.EMAPeriod = utils.MinInt(15, config.EMAPeriod+2)
	}

	return &adaptedConfig
}

// AdaptIndicatorParametersML корректирует параметры индикаторов на основе оптимизированных значений для разных режимов
func AdaptIndicatorParametersML(candles []models.Candle, config *models.Config) *models.Config {
	if !config.AdaptiveIndicator || len(candles) < 30 {
		return config
	}

	// Создаем копию конфига
	adaptedConfig := *config

	// Определяем рыночный режим
	regime := anomaly.EnhancedMarketRegimeClassification(candles)

	// Получаем матрицу адаптивных параметров
	params := getOptimizedParameters(regime.Type, regime.VolatilityLevel)

	// Применяем оптимизированные параметры
	adaptedConfig.RSIPeriod = params.RSI
	adaptedConfig.MACDFastPeriod = params.MACDFast
	adaptedConfig.MACDSlowPeriod = params.MACDSlow
	adaptedConfig.MACDSignalPeriod = params.MACDSignal
	adaptedConfig.BBPeriod = params.BB
	adaptedConfig.BBStdDev = params.BBStdDev
	adaptedConfig.EMAPeriod = params.EMA
	adaptedConfig.ADXPeriod = params.ADX

	// Точная настройка параметров на основе дополнительных факторов

	// Корректировка на основе соотношения волатильности
	atr5 := utils.CalculateATR(candles, 5)
	atr20 := utils.CalculateATR(candles, 20)
	volatilityRatio := 1.0
	if atr20 > 0 {
		volatilityRatio = atr5 / atr20
	}

	if volatilityRatio > 2.0 {
		// В экстремально волатильных условиях делаем индикаторы более отзывчивыми
		adaptedConfig.RSIPeriod = utils.MaxInt(5, adaptedConfig.RSIPeriod-2)
		adaptedConfig.EMAPeriod = utils.MaxInt(5, adaptedConfig.EMAPeriod-2)
	} else if volatilityRatio < 0.5 {
		// В условиях экстремально низкой волатильности используем более длинные периоды для уменьшения шума
		adaptedConfig.RSIPeriod = utils.MinInt(21, adaptedConfig.RSIPeriod+3)
		adaptedConfig.EMAPeriod = utils.MinInt(21, adaptedConfig.EMAPeriod+3)
	}

	return &adaptedConfig
}

// Оптимизированные параметры для разных рыночных режимов
type optimizedParams struct {
	RSI        int
	MACDFast   int
	MACDSlow   int
	MACDSignal int
	BB         int
	BBStdDev   float64
	EMA        int
	ADX        int
}

// Эти значения в идеале должны быть получены из процесса машинного обучения,
// который оптимизировал параметры для каждого типа режима
func getOptimizedParameters(regimeType, volatilityLevel string) optimizedParams {
	// Параметры по умолчанию
	params := optimizedParams{
		RSI:        14,
		MACDFast:   12,
		MACDSlow:   26,
		MACDSignal: 9,
		BB:         20,
		BBStdDev:   2.0,
		EMA:        10,
		ADX:        14,
	}

	// Режим-специфичные оптимизации
	switch regimeType {
	case "TRENDING":
		if volatilityLevel == "HIGH" {
			return optimizedParams{
				RSI:        9,
				MACDFast:   8,
				MACDSlow:   17,
				MACDSignal: 6,
				BB:         18,
				BBStdDev:   2.8,
				EMA:        8,
				ADX:        10,
			}
		} else {
			return optimizedParams{
				RSI:        11,
				MACDFast:   10,
				MACDSlow:   21,
				MACDSignal: 7,
				BB:         20,
				BBStdDev:   2.3,
				EMA:        9,
				ADX:        14,
			}
		}
	case "RANGING":
		if volatilityLevel == "LOW" {
			return optimizedParams{
				RSI:        21,
				MACDFast:   16,
				MACDSlow:   32,
				MACDSignal: 11,
				BB:         13,
				BBStdDev:   1.8,
				EMA:        18,
				ADX:        18,
			}
		} else {
			return optimizedParams{
				RSI:        16,
				MACDFast:   14,
				MACDSlow:   28,
				MACDSignal: 9,
				BB:         16,
				BBStdDev:   2.0,
				EMA:        14,
				ADX:        16,
			}
		}
	case "CHOPPY":
		return optimizedParams{
			RSI:        18,
			MACDFast:   16,
			MACDSlow:   32,
			MACDSignal: 12,
			BB:         12,
			BBStdDev:   1.6,
			EMA:        16,
			ADX:        21,
		}
	case "VOLATILE":
		return optimizedParams{
			RSI:        7,
			MACDFast:   6,
			MACDSlow:   13,
			MACDSignal: 5,
			BB:         24,
			BBStdDev:   3.2,
			EMA:        6,
			ADX:        8,
		}
	}

	// При любом другом неизвестном режиме возвращаем значения по умолчанию
	return params
}
