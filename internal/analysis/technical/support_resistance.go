package technical

import (
	"chi/Predictor/internal/config"
	"chi/Predictor/internal/model"
)

// CalculateAllIndicators computes all technical indicators for a set of candles
func CalculateAllIndicators(candles []model.Candle, config *config.Config) *model.TechnicalIndicators {
	if len(candles) < 5 {
		return nil
	}

	// If adaptive parameters are enabled, adjust config first
	computeConfig := config
	if config.AdaptiveIndicator {
		// In real implementation this would be the adaptIndicatorParameters
		// function from market package
		// computeConfig = adaptIndicatorParameters(candles, config)
	}

	// Calculate RSI
	rsi := CalculateRSI(candles, computeConfig.RSIPeriod)

	// Calculate MACD
	macd, macdSignal, macdHist := CalculateMACD(
		candles,
		computeConfig.MACDFastPeriod,
		computeConfig.MACDSlowPeriod,
		computeConfig.MACDSignalPeriod,
	)

	// Calculate Bollinger Bands
	bbUpper, bbMiddle, bbLower := CalculateBollingerBands(
		candles,
		computeConfig.BBPeriod,
		computeConfig.BBStdDev,
	)

	// Calculate EMA
	ema := CalculateEMA(candles, computeConfig.EMAPeriod)

	// Calculate ATR
	atr := CalculateATR(candles, computeConfig.ATRPeriod)

	// Calculate ADX
	adx, plusDI, minusDI := CalculateADX(candles, computeConfig.ADXPeriod)

	// Calculate Stochastic
	stochK, stochD := CalculateStochastic(candles, 14, 3)

	// Calculate OBV
	obv := CalculateOBV(candles)

	// Calculate volatility ratio
	atr5 := CalculateATR(candles, 5)
	atr20 := CalculateATR(candles, 20)
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
	trends := IdentifyTrends(candles, ema)

	// Identify support/resistance levels
	support, resistance := IdentifySupportResistance(candles)

	// Generate trade signal based on multiple indicators
	tradeSignal := DetermineTradeSignal(
		rsi, macd, macdSignal, macdHist,
		lastClose, bbUpper, bbMiddle, bbLower,
		stochK, stochD, adx, plusDI, minusDI,
		ema)

	return &model.TechnicalIndicators{
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
		Close:            lastClose, // Add the current price for convenience
	}
}

// DetermineTradeSignal generates a trade signal based on multiple indicators
func DetermineTradeSignal(rsi, macd, macdSignal, macdHist, price, bbUpper, bbMiddle, bbLower,
	stochK, stochD, adx, plusDI, minusDI, ema float64) string {

	// Count bullish and bearish signals
	bullishSignals := 0
	bearishSignals := 0

	// RSI signals
	if rsi < 30 {
		bullishSignals += 2 // Oversold
	} else if rsi < 40 {
		bullishSignals += 1 // Approaching oversold
	} else if rsi > 70 {
		bearishSignals += 2 // Overbought
	} else if rsi > 60 {
		bearishSignals += 1 // Approaching overbought
	}

	// MACD signals
	if macdHist > 0 && macdHist > macd*0.1 {
		bullishSignals += 1 // Positive histogram
		if macdHist > macd*0.2 && macd > 0 {
			bullishSignals += 1 // Strong positive histogram with positive MACD
		}
	} else if macdHist < 0 && macdHist < macd*0.1 {
		bearishSignals += 1 // Negative histogram
		if macdHist < macd*0.2 && macd < 0 {
			bearishSignals += 1 // Strong negative histogram with negative MACD
		}
	}

	// Bollinger Bands signals
	if price < bbLower {
		bullishSignals += 1 // Price below lower band (potential bounce)
	} else if price > bbUpper {
		bearishSignals += 1 // Price above upper band (potential reversal)
	}

	// Stochastic signals
	if stochK < 20 && stochD < 20 && stochK > stochD {
		bullishSignals += 1 // Oversold and turning up
	} else if stochK > 80 && stochD > 80 && stochK < stochD {
		bearishSignals += 1 // Overbought and turning down
	}

	// ADX and Directional Movement
	if adx > 25 {
		if plusDI > minusDI {
			bullishSignals += 1 // Strong uptrend
		} else {
			bearishSignals += 1 // Strong downtrend
		}
	}

	// EMA signal
	if price > ema {
		bullishSignals += 1 // Price above EMA (bullish)
	} else if price < ema {
		bearishSignals += 1 // Price below EMA (bearish)
	}

	// Determine signal based on signal counts
	netSignal := bullishSignals - bearishSignals

	if netSignal >= 4 {
		return "STRONG_BUY"
	} else if netSignal >= 2 {
		return "BUY"
	} else if netSignal <= -4 {
		return "STRONG_SELL"
	} else if netSignal <= -2 {
		return "SELL"
	} else {
		return "NEUTRAL"
	}
}
