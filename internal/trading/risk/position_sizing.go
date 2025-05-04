package risk

import (
	"chi/Predictor/internal/model"
	"math"
)

// PositionSizingResult holds position sizing calculation results
type PositionSizingResult struct {
	PositionSize    float64 `json:"position_size"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
	AccountRisk     float64 `json:"account_risk"`
}

// DetermineStopLoss calculates an appropriate stop-loss level based on technical indicators
func DetermineStopLoss(candles []model.Candle, indicators *model.TechnicalIndicators, direction string) float64 {
	currentPrice := candles[len(candles)-1].Close
	atr := indicators.ATR

	// Base calculation based on ATR
	atrMultiplier := 1.5

	defaultStop := 0.0
	if direction == "UP" {
		defaultStop = currentPrice - (atr * atrMultiplier)
	} else {
		defaultStop = currentPrice + (atr * atrMultiplier)
	}

	return defaultStop
}

// CalculatePositionSize determines the appropriate position size based on risk parameters
func CalculatePositionSize(currentPrice float64, stopLoss float64, accountSize float64, riskPerTrade float64) *PositionSizingResult {
	// Calculate stop size in points
	stopSizePoints := math.Abs(currentPrice - stopLoss)

	// Calculate risk amount in money
	riskAmount := accountSize * riskPerTrade

	// Position size = Risk amount / Stop size in money
	positionSize := riskAmount / stopSizePoints

	// Determine take-profit level with a 1:2 risk-reward ratio
	takeProfit := 0.0
	if currentPrice > stopLoss {
		// Long position
		takeProfit = currentPrice + (currentPrice-stopLoss)*2.0
	} else {
		// Short position
		takeProfit = currentPrice - (stopLoss-currentPrice)*2.0
	}

	// Calculate risk-reward ratio
	riskRewardRatio := 0.0
	if stopSizePoints > 0 {
		riskRewardRatio = math.Abs(takeProfit-currentPrice) / stopSizePoints
	}

	return &PositionSizingResult{
		PositionSize:    positionSize,
		StopLoss:        stopLoss,
		TakeProfit:      takeProfit,
		RiskRewardRatio: riskRewardRatio,
		AccountRisk:     riskPerTrade,
	}
}

// AdjustPositionSizeForVolatility modifies position size based on market volatility
func AdjustPositionSizeForVolatility(baseSize float64, volatilityRatio float64) float64 {
	// Reduce position size in high-volatility markets
	if volatilityRatio > 1.5 {
		return baseSize * (1 / volatilityRatio)
	}

	// Increase position size slightly in low-volatility markets
	if volatilityRatio < 0.7 {
		return baseSize * 1.2
	}

	return baseSize
}
