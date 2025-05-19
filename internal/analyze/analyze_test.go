package analyze

import (
	"testing"
	"time"

	"github.com/Alias1177/Predictor/models"
)

func TestDetermineMarketRegime(t *testing.T) {
	tests := []struct {
		name     string
		candles  []models.Candle
		expected string
	}{
		{
			name: "Недостаточно данных",
			candles: []models.Candle{
				{Close: 100, Volume: 1000},
			},
			expected: "NEUTRAL",
		},
		{
			name: "Волатильный бычий тренд",
			candles: generateTestCandles(50, func(i int) models.Candle {
				return models.Candle{
					Close:     100 + float64(i)*2 + float64(i%3)*5,
					High:      105 + float64(i)*2 + float64(i%3)*5,
					Low:       95 + float64(i)*2 + float64(i%3)*5,
					Volume:    int64(1000 + i*100),
					Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				}
			}),
			expected: "VOLATILE_BULLISH",
		},
		{
			name: "Волатильный медвежий тренд",
			candles: generateTestCandles(50, func(i int) models.Candle {
				return models.Candle{
					Close:     100 - float64(i)*2 - float64(i%3)*5,
					High:      105 - float64(i)*2 - float64(i%3)*5,
					Low:       95 - float64(i)*2 - float64(i%3)*5,
					Volume:    int64(1000 + i*100),
					Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				}
			}),
			expected: "VOLATILE_BEARISH",
		},
		{
			name: "Боковой тренд",
			candles: generateTestCandles(50, func(i int) models.Candle {
				return models.Candle{
					Close:     100 + float64(i%3)*2,
					High:      102 + float64(i%3)*2,
					Low:       98 + float64(i%3)*2,
					Volume:    int64(1000),
					Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				}
			}),
			expected: "RANGING",
		},
		{
			name: "Накопление",
			candles: generateTestCandles(50, func(i int) models.Candle {
				return models.Candle{
					Close:     100 + float64(i)*0.5,
					High:      102 + float64(i)*0.5,
					Low:       98 + float64(i)*0.5,
					Volume:    int64(1000 + i*200),
					Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				}
			}),
			expected: "ACCUMULATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineMarketRegime(tt.candles)
			if result != tt.expected {
				t.Errorf("determineMarketRegime() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func generateTestCandles(n int, generator func(int) models.Candle) []models.Candle {
	candles := make([]models.Candle, n)
	for i := 0; i < n; i++ {
		candles[i] = generator(i)
	}
	return candles
}
