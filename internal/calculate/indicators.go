package calculate

import (
	"github.com/Alias1177/Predictor/internal/patterns"
	"github.com/Alias1177/Predictor/internal/utils"
	"github.com/Alias1177/Predictor/models"
)

// calculateAllIndicators calculates all technical indicators
func CalculateAllIndicators(candles []models.Candle, config *models.Config) *models.TechnicalIndicators {
	if len(candles) < 5 {
		return nil
	}

	// If adaptive parameters are enabled, adjust config first
	computeConfig := config
	if config.AdaptiveIndicator {
		computeConfig = AdaptIndicatorParameters(candles, config)
	}

	// Calculate RSI
	rsi := calculateRSI(candles, computeConfig.RSIPeriod)

	// Calculate MACD
	macd, macdSignal, macdHist := calculateMACD(
		candles,
		computeConfig.MACDFastPeriod,
		computeConfig.MACDSlowPeriod,
		computeConfig.MACDSignalPeriod,
	)

	// Calculate Bollinger Bands
	bbUpper, bbMiddle, bbLower := calculateBollingerBands(
		candles,
		computeConfig.BBPeriod,
		computeConfig.BBStdDev,
	)

	// Calculate EMA
	ema := patterns.CalculateEMA(candles, computeConfig.EMAPeriod)

	// Calculate ATR
	atr := utils.CalculateATR(candles, computeConfig.ATRPeriod)

	// Calculate ADX
	adx, plusDI, minusDI := utils.CalculateADX(candles, computeConfig.ADXPeriod)

	// Calculate Stochastic
	stochK, stochD := calculateStochastic(candles, 14, 3)

	// Calculate OBV
	obv := calculateOBV(candles)

	// Calculate volatility ratio
	atr5 := utils.CalculateATR(candles, 5)
	atr20 := utils.CalculateATR(candles, 20)
	volatilityRatio := atr5 / atr20

	// Price change percentage over last 5 candles
	firstClose := candles[len(candles)-5].Close
	lastClose := candles[len(candles)-1].Close
	priceChangePct := (lastClose - firstClose) / firstClose * 100

	// Volume change if volume data is available
	var volumeChangePct float64
	if len(candles) > 5 && candles[len(candles)-1].Volume > 0 && candles[len(candles)-5].Volume > 0 {
		firstVolume := float64(candles[len(candles)-5].Volume)
		lastVolume := float64(candles[len(candles)-1].Volume)
		volumeChangePct = (lastVolume - firstVolume) / firstVolume * 100
	}

	// Calculate momentum (difference between current close and n periods ago)
	momentum := 0.0
	if len(candles) > 10 {
		momentum = lastClose - candles[len(candles)-10].Close
	}

	// Identify trends
	trends := patterns.IdentifyTrends(candles, ema)

	// Identify support/resistance levels
	support, resistance := identifySupportResistance(candles)

	// Generate trade signal based on multiple indicators
	tradeSignal := DetermineTradeSignal(
		rsi, macd, macdSignal, macdHist,
		lastClose, bbUpper, bbMiddle, bbLower,
		stochK, stochD, adx, plusDI, minusDI,
		ema)

	return &models.TechnicalIndicators{
		RSI:              rsi,
		MACD:             macd,
		MACDSignal:       macdSignal,
		MACDHist:         macdHist,
		BBUpper:          bbUpper,
		BBMiddle:         bbMiddle,
		BBLower:          bbLower,
		EMA:              ema,
		ATR:              atr,
		ADX:              adx,
		PlusDI:           plusDI,
		MinusDI:          minusDI,
		PriceChange:      priceChangePct,
		VolumeChange:     volumeChangePct,
		Momentum:         momentum,
		Trends:           trends,
		Support:          support,
		Resistance:       resistance,
		Stochastic:       stochK,
		StochasticSignal: stochD,
		OBV:              obv,
		VolatilityRatio:  volatilityRatio,
		TradeSignal:      tradeSignal,
	}
}
