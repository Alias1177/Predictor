package models

// AnomalyDetection представляет обнаруженные аномалии на рынке
type AnomalyDetection struct {
	IsAnomaly    bool
	AnomalyScore float64
	Type         string
	Description  string
}
