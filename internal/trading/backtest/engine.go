package backtest

import (
	"chi/Predictor/internal/analysis/market"
	"chi/Predictor/internal/analysis/prediction"
	"chi/Predictor/internal/analysis/technical"
	"chi/Predictor/internal/api/twelvedata"
	"chi/Predictor/internal/config"
	"chi/Predictor/internal/model"
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// Engine handles backtesting operations
type Engine struct {
	client       *twelvedata.Client
	config       *config.Config
	initialValue float64
}

// NewEngine creates a new backtesting engine
func NewEngine(client *twelvedata.Client, cfg *config.Config) *Engine {
	return &Engine{
		client:       client,
		config:       cfg,
		initialValue: 10000.0, // Default initial account value
	}
}

// SetInitialValue sets the initial capital for backtesting
func (e *Engine) SetInitialValue(value float64) {
	e.initialValue = value
}

// Run executes a backtest over historical data
func (e *Engine) Run(ctx context.Context, days int) (*model.BacktestResults, error) {
	// Fetch historical candles
	historicalCandles, err := e.client.GetHistoricalCandles(ctx, e.config.Symbol, e.config.Interval, days)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical data: %w", err)
	}

	if len(historicalCandles) < 100 {
		return nil, fmt.Errorf("insufficient historical data for backtesting, got %d candles", len(historicalCandles))
	}

	// Initialize results
	results := &model.BacktestResults{
		MaxConsecutive: struct {
			Wins  int `json:"wins"`
			Loses int `json:"loses"`
		}{},
		MarketRegimePerformance: make(map[string]float64),
		TimeframePerformance:    make(map[string]float64),
		DetailedResults:         []model.PredictionResult{},
		MonthlyReturns:          make(map[string]float64),
	}

	// Track profit and loss
	var totalProfit, totalLoss float64

	// Configure test parameters
	windowSize := e.config.CandleCount
	predictionInterval := 1 // How many candles ahead to validate

	// Set validation limit
	validationLimit := len(historicalCandles) - predictionInterval
	consecutiveWins := 0
	consecutiveLosses := 0

	// Track market regime statistics
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

	// Track account balance for equity curve
	accountBalance := e.initialValue
	balanceHistory := []float64{accountBalance}
	highWaterMark := accountBalance
	maxDrawdown := 0.0

	// Iterate through historical data
	for i := windowSize; i < validationLimit; i += predictionInterval {
		// Extract test window
		testWindow := historicalCandles[i-windowSize : i]

		// Calculate indicators
		indicators := technical.CalculateAllIndicators(testWindow, e.config)

		// Get market regime and anomalies
		regime := market.ClassifyMarketRegime(testWindow)
		anomaly := market.DetectMarketAnomalies(testWindow)

		// Create MTF data for testing
		mtfData := map[string][]model.Candle{
			"5min": testWindow,
		}

		// Generate prediction
		direction, confidence, score, factors := prediction.GeneratePrediction(
			testWindow, indicators, mtfData, regime, anomaly, e.config)

		// Create prediction record
		predTime := time.Now().Add(-time.Duration(validationLimit-i) * time.Minute * 5)
		targetTime := time.Now().Add(-time.Duration(validationLimit-i-predictionInterval) * time.Minute * 5)

		result := model.PredictionResult{
			Direction:        direction,
			Confidence:       confidence,
			Score:            score,
			Factors:          factors,
			Timestamp:        predTime,
			PredictionID:     fmt.Sprintf("BT-%d", i),
			PredictionTarget: targetTime,
		}

		// Filter signals with low confidence
		if direction == "NEUTRAL" || (confidence != "HIGH" && math.Abs(score) < 0.3) {
			continue // Skip trades with low confidence
		}

		// Filter based on market regime
		if regime.Type == "CHOPPY" && regime.Strength > 0.7 {
			continue // Skip highly chaotic markets
		}

		// Determine price values for validation
		currentPrice := testWindow[len(testWindow)-1].Close
		futurePrice := historicalCandles[i+predictionInterval].Close
		priceChange := futurePrice - currentPrice

		// Determine actual outcome
		actualOutcome := "NEUTRAL"
		if priceChange > 0 {
			actualOutcome = "UP"
		} else if priceChange < 0 {
			actualOutcome = "DOWN"
		}

		result.ActualOutcome = actualOutcome

		// Check if prediction was correct
		wasCorrect := direction == actualOutcome
		result.WasCorrect = wasCorrect

		// Add to detailed results
		results.DetailedResults = append(results.DetailedResults, result)
		results.TotalTrades++

		// Update counters and account balance
		if wasCorrect {
			results.WinningTrades++
			consecutiveWins++
			consecutiveLosses = 0

			// Calculate profit
			profit := math.Abs(priceChange) * 10000 // Scale for pip value
			totalProfit += profit
			accountBalance += profit
		} else {
			results.LosingTrades++
			consecutiveLosses++
			consecutiveWins = 0

			// Calculate loss
			loss := math.Abs(priceChange) * 10000 // Scale for pip value
			totalLoss += loss
			accountBalance -= loss
		}

		// Track balance and drawdown
		balanceHistory = append(balanceHistory, accountBalance)
		if accountBalance > highWaterMark {
			highWaterMark = accountBalance
		} else {
			currentDrawdown := (highWaterMark - accountBalance) / highWaterMark
			if currentDrawdown > maxDrawdown {
				maxDrawdown = currentDrawdown
			}
		}

		// Update consecutive counters
		if consecutiveWins > results.MaxConsecutive.Wins {
			results.MaxConsecutive.Wins = consecutiveWins
		}
		if consecutiveLosses > results.MaxConsecutive.Loses {
			results.MaxConsecutive.Loses = consecutiveLosses
		}

		// Update market regime statistics
		if stats, exists := regimeStats[regime.Type]; exists {
			stats.total++
			if wasCorrect {
				stats.correct++
			}
			regimeStats[regime.Type] = stats
		}

		// Track monthly returns
		month := result.Timestamp.Format("2006-01")
		if result.WasCorrect {
			if _, ok := results.MonthlyReturns[month]; ok {
				results.MonthlyReturns[month] += profit
			} else {
				results.MonthlyReturns[month] = profit
			}
		} else {
			if _, ok := results.MonthlyReturns[month]; ok {
				results.MonthlyReturns[month] -= loss
			} else {
				results.MonthlyReturns[month] = -loss
			}
		}
	}

	// Calculate metrics
	e.calculateMetrics(results, totalProfit, totalLoss, regimeStats, balanceHistory, maxDrawdown)

	return results, nil
}

// calculateMetrics computes performance metrics for the backtest
func (e *Engine) calculateMetrics(
	results *model.BacktestResults,
	totalProfit, totalLoss float64,
	regimeStats map[string]struct{ correct, total int },
	balanceHistory []float64,
	maxDrawdown float64) {

	// Win percentage
	if results.TotalTrades > 0 {
		results.WinPercentage = float64(results.WinningTrades) / float64(results.TotalTrades) * 100
	}

	// Average gain and loss
	if results.WinningTrades > 0 {
		results.AverageGain = totalProfit / float64(results.WinningTrades)
	}
	if results.LosingTrades > 0 {
		results.AverageLoss = totalLoss / float64(results.LosingTrades)
	}

	// Profit factor
	if totalLoss > 0 {
		results.ProfitFactor = totalProfit / totalLoss
	} else {
		results.ProfitFactor = totalProfit // If no losses
	}

	// Maximum drawdown
	results.MaxDrawdown = maxDrawdown * 100 // Convert to percentage

	// Market regime performance
	for regime, stats := range regimeStats {
		if stats.total > 0 {
			results.MarketRegimePerformance[regime] = float64(stats.correct) / float64(stats.total) * 100
		}
	}

	// Equity growth percentage
	if len(balanceHistory) > 0 {
		initialBalance := balanceHistory[0]
		finalBalance := balanceHistory[len(balanceHistory)-1]
		results.EquityGrowthPercent = ((finalBalance - initialBalance) / initialBalance) * 100
	}

	// Store equity curve
	results.EquityCurve = balanceHistory

	// Convert monthly returns to percentages
	initialBalance := e.initialValue
	for month, value := range results.MonthlyReturns {
		results.MonthlyReturns[month] = (value / initialBalance) * 100
	}

	// Calculate Sharpe ratio (simplified)
	if len(results.DetailedResults) > 0 {
		var returns []float64
		for i := 1; i < len(balanceHistory); i++ {
			periodReturn := (balanceHistory[i] - balanceHistory[i-1]) / balanceHistory[i-1]
			returns = append(returns, periodReturn)
		}

		mean := calculateMean(returns)
		stdDev := calculateStdDev(returns, mean)

		if stdDev > 0 {
			results.SharpeRatio = mean / stdDev * math.Sqrt(252) // Annualized Sharpe
		}
	}

	// Calculate percentage metrics
	basePrice := 0.0
	if len(results.DetailedResults) > 0 {
		// Get reference price for percentage calculations
		baseCount := 20
		if len(results.DetailedResults) < baseCount {
			baseCount = len(results.DetailedResults)
		}

		for i := 0; i < baseCount; i++ {
			// This is just a proxy since we don't store the price in PredictionResult
			// In a real implementation, you would use the actual price
			basePrice += float64(i + 100) // Dummy value
		}
		basePrice /= float64(baseCount)
	}

	if basePrice > 0 {
		// Convert pip values to percentages
		results.AverageGainPercent = (results.AverageGain / 10000) * 100
		results.AverageLossPercent = (results.AverageLoss / 10000) * 100
	}

	// Calculate total return percentage
	initialBalance := e.initialValue
	finalBalance := initialBalance +
		(float64(results.WinningTrades) * results.AverageGain) -
		(float64(results.LosingTrades) * results.AverageLoss)

	results.TotalReturnPercent = ((finalBalance - initialBalance) / initialBalance) * 100
}

// Helper functions
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}

	var sumSquaredDiff float64
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}

	return math.Sqrt(sumSquaredDiff / float64(len(values)-1))
}

// FormatResults creates a human-readable summary of backtest results
func (e *Engine) FormatResults(results *model.BacktestResults) string {
	if results == nil {
		return "No backtest results available"
	}

	output := "\n===== BACKTEST RESULTS =====\n"
	output += fmt.Sprintf("Total trades: %d\n", results.TotalTrades)
	output += fmt.Sprintf("Winning trades: %d (%.2f%%)\n", results.WinningTrades, results.WinPercentage)
	output += fmt.Sprintf("Total return: %.2f%%\n", results.TotalReturnPercent)
	output += fmt.Sprintf("Average gain: %.2f pips (%.2f%%)\n",
		results.AverageGain, results.AverageGainPercent)
	output += fmt.Sprintf("Average loss: %.2f pips (%.2f%%)\n",
		results.AverageLoss, results.AverageLossPercent)

	output += fmt.Sprintf("Profit factor: %.2f\n", results.ProfitFactor)
	output += fmt.Sprintf("Maximum drawdown: %.2f%%\n", results.MaxDrawdown)
	output += fmt.Sprintf("Max consecutive wins: %d\n", results.MaxConsecutive.Wins)
	output += fmt.Sprintf("Max consecutive losses: %d\n", results.MaxConsecutive.Loses)

	output += "\nPerformance by market regime:\n"
	for regime, performance := range results.MarketRegimePerformance {
		if performance > 0 {
			output += fmt.Sprintf("- %s: %.2f%%\n", regime, performance)
		}
	}

	if len(results.MonthlyReturns) > 0 {
		output += "\nMonthly returns:\n"

		// Sort months for chronological display
		months := make([]string, 0, len(results.MonthlyReturns))
		for month := range results.MonthlyReturns {
			months = append(months, month)
		}
		sort.Strings(months)

		for _, month := range months {
			returnValue := results.MonthlyReturns[month]
			sign := ""
			if returnValue > 0 {
				sign = "+"
			}
			output += fmt.Sprintf("- %s: %s%.2f%%\n", month, sign, returnValue)
		}
	}

	output += fmt.Sprintf("\nTotal equity growth: %.2f%%\n", results.EquityGrowthPercent)

	return output
}
