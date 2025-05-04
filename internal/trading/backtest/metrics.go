package backtest

import (
	"chi/Predictor/internal/model"
	"math"
	"time"
)

// CalculatePerformanceMetrics computes detailed performance metrics from backtest results
func CalculatePerformanceMetrics(results *model.BacktestResults) {
	if results == nil || results.TotalTrades == 0 {
		return
	}

	// Calculate Sharpe ratio
	calculateSharpeRatio(results)

	// Calculate drawdown metrics
	calculateDrawdownMetrics(results)

	// Calculate gain/loss ratio
	if results.AverageLoss > 0 {
		results.ProfitFactor = results.AverageGain / results.AverageLoss
	}

	// Calculate monthly stats if we have detailed results
	if len(results.DetailedResults) > 0 {
		calculateMonthlyStats(results)
	}
}

// calculateSharpeRatio computes the Sharpe ratio based on trade returns
func calculateSharpeRatio(results *model.BacktestResults) {
	if len(results.DetailedResults) == 0 {
		return
	}

	// Extract returns from trades
	var returns []float64
	for _, trade := range results.DetailedResults {
		var returnPct float64
		if trade.WasCorrect {
			returnPct = 0.01 // Assume 1% return per winning trade
		} else {
			returnPct = -0.01 // Assume -1% return per losing trade
		}
		returns = append(returns, returnPct)
	}

	// Calculate mean return
	meanReturn := 0.0
	for _, r := range returns {
		meanReturn += r
	}
	meanReturn /= float64(len(returns))

	// Calculate standard deviation of returns
	variance := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)
	stdDev := math.Sqrt(variance)

	// Calculate Sharpe ratio (assuming 0% risk-free rate)
	if stdDev > 0 {
		// Annualized Sharpe ratio (assuming daily returns and 252 trading days)
		// Will need adjustment based on the actual timeframe of your data
		timeframe := getTimeframeMultiplier(results.DetailedResults)
		results.SharpeRatio = (meanReturn / stdDev) * math.Sqrt(timeframe)
	}

	// Monthly Sharpe (Sharpe calculated on monthly basis)
	monthlyReturns := calculateMonthlyReturnsSeries(results)
	if len(monthlyReturns) > 0 {
		monthlyMean := mean(monthlyReturns)
		monthlyStdDev := stdDev(monthlyReturns, monthlyMean)
		if monthlyStdDev > 0 {
			// Annualized from monthly data (multiply by sqrt(12))
			results.MonthlySharpe = monthlyMean / monthlyStdDev * math.Sqrt(12)
		}
	}
}

// calculateDrawdownMetrics computes maximum drawdown and related metrics
func calculateDrawdownMetrics(results *model.BacktestResults) {
	if len(results.DetailedResults) == 0 {
		return
	}

	// Rebuild equity curve if not present
	if len(results.EquityCurve) == 0 {
		initialEquity := 10000.0 // Start with hypothetical $10,000
		equity := initialEquity
		results.EquityCurve = append(results.EquityCurve, equity)

		for _, trade := range results.DetailedResults {
			if trade.WasCorrect {
				equity += results.AverageGain
			} else {
				equity -= results.AverageLoss
			}
			results.EquityCurve = append(results.EquityCurve, equity)
		}
	}

	// Calculate maximum drawdown
	maxDrawdown := 0.0
	peak := results.EquityCurve[0]

	for _, equity := range results.EquityCurve {
		if equity > peak {
			peak = equity
		}
		drawdown := (peak - equity) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	results.MaxDrawdown = maxDrawdown * 100 // Convert to percentage
}

// calculateMonthlyStats aggregates performance metrics by month
func calculateMonthlyStats(results *model.BacktestResults) {
	// Initialize monthly returns map if not present
	if results.MonthlyReturns == nil {
		results.MonthlyReturns = make(map[string]float64)
	}

	// Group trades by month
	monthlyTrades := make(map[string]struct {
		wins   int
		losses int
		pnl    float64
	})

	for _, trade := range results.DetailedResults {
		month := trade.Timestamp.Format("2006-01")
		stats := monthlyTrades[month]

		if trade.WasCorrect {
			stats.wins++
			stats.pnl += results.AverageGain
		} else {
			stats.losses++
			stats.pnl -= results.AverageLoss
		}

		monthlyTrades[month] = stats
	}

	// Calculate monthly returns
	initialEquity := 10000.0 // Start with hypothetical $10,000
	for month, stats := range monthlyTrades {
		monthlyReturn := (stats.pnl / initialEquity) * 100
		results.MonthlyReturns[month] = monthlyReturn
	}
}

// calculateMonthlyReturnsSeries creates a series of monthly returns for Sharpe calculation
func calculateMonthlyReturnsSeries(results *model.BacktestResults) []float64 {
	var monthlyReturns []float64

	for _, return_ := range results.MonthlyReturns {
		monthlyReturns = append(monthlyReturns, return_/100) // Convert from % to decimal
	}

	return monthlyReturns
}

// getTimeframeMultiplier returns the annualization factor based on timeframe
func getTimeframeMultiplier(trades []model.PredictionResult) float64 {
	if len(trades) < 2 {
		return 252.0 // Default to daily (252 trading days)
	}

	// Calculate average time difference between trades
	var totalDuration time.Duration
	for i := 1; i < len(trades); i++ {
		diff := trades[i].Timestamp.Sub(trades[i-1].Timestamp)
		if diff > 0 {
			totalDuration += diff
		}
	}

	avgDuration := totalDuration / time.Duration(len(trades)-1)
	durationHours := avgDuration.Hours()

	if durationHours <= 1 {
		// Hourly data (approximately 252 * 6.5 periods per year)
		return 252.0 * 6.5
	} else if durationHours <= 24 {
		// Daily data (approximately 252 periods per year)
		return 252.0
	} else if durationHours <= 24*7 {
		// Weekly data (approximately 52 periods per year)
		return 52.0
	} else {
		// Monthly data (12 periods per year)
		return 12.0
	}
}

// Helper functions
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func stdDev(values []float64, mean float64) float64 {
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
