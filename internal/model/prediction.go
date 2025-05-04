package model

import "time"

// PredictionResult stores the outcome of a prediction
type PredictionResult struct {
	Direction        string    `json:"direction"`
	Confidence       string    `json:"confidence"`
	Score            float64   `json:"score"`
	Factors          []string  `json:"factors"`
	Timestamp        time.Time `json:"timestamp"`
	PredictionID     string    `json:"prediction_id"`
	PredictionTarget time.Time `json:"prediction_target"` // When this prediction should be validated
	ActualOutcome    string    `json:"actual_outcome,omitempty"`
	WasCorrect       bool      `json:"was_correct,omitempty"`
}
