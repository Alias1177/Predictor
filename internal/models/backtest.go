package models

// BacktestResult представляет результат бэктестинга
type BacktestResult struct {
	DetailedResults []PredictionResult
}

// PredictionResult представляет результат предсказания для бэктестинга
type PredictionResult struct {
	WasCorrect bool
	Factors    []string
}
