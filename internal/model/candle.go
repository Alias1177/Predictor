package model

// Candle represents a single price candle
type Candle struct {
	Datetime string  `json:"datetime"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   int64   `json:"volume,omitempty"`
}

// TwelveResponse represents the API response from Twelve Data
type TwelveResponse struct {
	Meta struct {
		Symbol   string `json:"symbol"`
		Interval string `json:"interval"`
	} `json:"meta"`
	Values []struct {
		Datetime string  `json:"datetime"`
		Open     float64 `json:"open,string"`
		High     float64 `json:"high,string"`
		Low      float64 `json:"low,string"`
		Close    float64 `json:"close,string"`
		Volume   int64   `json:"volume,string,omitempty"`
	} `json:"values"`
	Status string `json:"status"`
}
