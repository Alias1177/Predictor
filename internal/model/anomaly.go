package model

// AnomalyDetection contains information about market anomalies
type AnomalyDetection struct {
	IsAnomaly        bool     `json:"is_anomaly"`
	AnomalyType      string   `json:"anomaly_type,omitempty"` // PRICE_SPIKE, VOLUME_SPIKE, GAP, PATTERN_BREAK
	AnomalyScore     float64  `json:"anomaly_score"`          // 0-1 score
	Details          string   `json:"details,omitempty"`
	RecommendedFlags []string `json:"recommended_flags,omitempty"`
}
