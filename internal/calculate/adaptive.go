package calculate

import (
	"github.com/Alias1177/Predictor/internal/anomaly"
	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
	"math"
)

// adaptIndicatorParameters dynamically adjusts indicator parameters based on market conditions
func AdaptIndicatorParameters(candles []models.Candle, config *models.Config) *models.Config {
	if !config.AdaptiveIndicator || len(candles) < 30 {
		return config // Return original config if adaptation is disabled or not enough data
	}

	// Create a copy of the config
	adaptedConfig := *config

	// Calculate volatility measures
	atr5 := utils.CalculateATR(candles, 5)
	atr20 := utils.CalculateATR(candles, 20)
	volatilityRatio := atr5 / atr20

	// Check market regime
	regime := anomaly.EnhancedMarketRegimeClassification(candles)

	// Adjust RSI period based on volatility
	if volatilityRatio > 1.5 {
		// Higher volatility - use shorter periods for faster reaction
		adaptedConfig.RSIPeriod = utils.MaxInt(5, config.RSIPeriod-2)
	} else if volatilityRatio < 0.7 {
		// Lower volatility - use longer periods to reduce noise
		adaptedConfig.RSIPeriod = utils.MinInt(14, config.RSIPeriod+2)
	}

	// Adjust Bollinger Bands parameters based on market regime
	if regime.Type == "TRENDING" && regime.Strength > 0.7 {
		// In strong trends, widen the bands
		adaptedConfig.BBStdDev = math.Min(3.0, config.BBStdDev+0.3)
	} else if regime.Type == "RANGING" {
		// In range-bound markets, tighten the bands
		adaptedConfig.BBStdDev = math.Max(1.8, config.BBStdDev-0.3)
	}

	// Adjust MACD parameters based on market regime
	if regime.Type == "CHOPPY" || regime.Type == "RANGING" {
		// In non-trending markets, use wider MACD settings
		adaptedConfig.MACDFastPeriod = utils.MinInt(12, config.MACDFastPeriod+2)
		adaptedConfig.MACDSlowPeriod = utils.MinInt(26, config.MACDSlowPeriod+3)
	} else if regime.Type == "TRENDING" && regime.Strength > 0.6 {
		// In trending markets, use more responsive MACD settings
		adaptedConfig.MACDFastPeriod = utils.MaxInt(5, config.MACDFastPeriod-1)
		adaptedConfig.MACDSlowPeriod = utils.MaxInt(12, config.MACDSlowPeriod-2)
	}

	// Adjust EMA period based on momentum
	if regime.MomentumStrength > 0.7 {
		// Strong momentum - use shorter EMA period
		adaptedConfig.EMAPeriod = utils.MaxInt(8, config.EMAPeriod-2)
	} else if regime.MomentumStrength < 0.3 {
		// Weak momentum - use longer EMA period
		adaptedConfig.EMAPeriod = utils.MinInt(15, config.EMAPeriod+2)
	}

	return &adaptedConfig
}
