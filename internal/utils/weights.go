package utils

import (
	"math"
	"time"

	"github.com/Alias1177/Predictor/models"
)

// FactorWeight представляет вес и корреляцию фактора
type FactorWeight struct {
	Weight      float64
	Correlation float64
	LastUpdate  time.Time
}

var factorWeights = map[string]FactorWeight{
	"TREND":   {1.5, 0.0, time.Now()},
	"RSI":     {1.0, 0.0, time.Now()},
	"MACD":    {1.2, 0.0, time.Now()},
	"BB":      {1.0, 0.0, time.Now()},
	"VOLUME":  {1.3, 0.0, time.Now()},
	"PATTERN": {1.8, 0.0, time.Now()},
}

// UpdateFactorWeights обновляет веса на основе исторических данных
func UpdateFactorWeights(results []models.PredictionResult) {
	for _, result := range results {
		for _, factor := range result.Factors {
			if weight, exists := factorWeights[factor]; exists {
				// Обновляем корреляцию на основе успешности предсказания
				if result.WasCorrect {
					weight.Correlation += 0.1
				} else {
					weight.Correlation -= 0.1
				}
				weight.Correlation = math.Max(-1.0, math.Min(1.0, weight.Correlation))

				// Обновляем вес с учетом корреляции
				weight.Weight *= (1.0 + weight.Correlation*0.1)
				weight.LastUpdate = time.Now()
				factorWeights[factor] = weight
			}
		}
	}
}

// GetFactorWeight возвращает вес фактора
func GetFactorWeight(factor string) float64 {
	if weight, exists := factorWeights[factor]; exists {
		return weight.Weight
	}
	return 1.0 // значение по умолчанию
}
