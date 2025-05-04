package analyze

// determineTradeSignal generates a trade signal based on multiple indicators
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
