package model

// BacktestResults stores backtesting results
type BacktestResults struct {
	TotalTrades    int     `json:"total_trades"`
	WinningTrades  int     `json:"winning_trades"`
	LosingTrades   int     `json:"losing_trades"`
	WinPercentage  float64 `json:"win_percentage"`
	AverageGain    float64 `json:"average_gain"`
	AverageLoss    float64 `json:"average_loss"`
	MaxConsecutive struct {
		Wins  int `json:"wins"`
		Loses int `json:"loses"`
	} `json:"max_consecutive"`
	MarketRegimePerformance map[string]float64 `json:"market_regime_performance"`
	TimeframePerformance    map[string]float64 `json:"timeframe_performance"`
	DetailedResults         []PredictionResult `json:"detailed_results"`
	ProfitFactor            float64            `json:"profit_factor"`
	MaxDrawdown             float64            `json:"max_drawdown"`
	SharpeRatio             float64            `json:"sharpe_ratio"`
	EquityCurve             []float64          `json:"equity_curve,omitempty"`
	EquityGrowthPercent     float64            `json:"equity_growth_percent"` // Рост капитала в %
	MonthlySharpe           float64            `json:"monthly_sharpe"`        // Месячный коэф. Шарпа
	MonthlyReturns          map[string]float64 `json:"monthly_returns"`
	AverageGainPercent      float64            `json:"average_gain_percent"`
	AverageLossPercent      float64            `json:"average_loss_percent"`
	TotalReturnPercent      float64            `json:"total_return_percent"`
}
