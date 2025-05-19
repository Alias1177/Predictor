package models

// Config содержит все настройки для анализа
type Config struct {
	TwelveAPIKey      string
	OpenAIAPIKey      string
	Symbol            string
	Interval          string
	CandleCount       int
	RSIPeriod         int
	MACDFastPeriod    int
	MACDSlowPeriod    int
	MACDSignalPeriod  int
	BBPeriod          int
	BBStdDev          float64
	EMAPeriod         int
	ADXPeriod         int
	ATRPeriod         int
	RequestTimeout    int
	AdaptiveIndicator bool
	EnableBacktest    bool
}
