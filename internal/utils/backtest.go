package utils

import (
	"context"

	"github.com/Alias1177/Predictor/models"
)

// RunBacktest выполняет бэктестинг стратегии
func RunBacktest(ctx context.Context, candles []models.Candle, cfg *models.Config) (*models.BacktestResults, error) {
	// TODO: Реализовать бэктестинг
	return &models.BacktestResults{
		DetailedResults: []models.PredictionResult{},
	}, nil
}
