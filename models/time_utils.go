package models

func CalculateCandlesForBacktest(interval string, days int) int {
	candlesPerDay := 0

	switch interval {
	case "1min":
		candlesPerDay = 24 * 60
	case "5min":
		candlesPerDay = 24 * 12
	case "15min":
		candlesPerDay = 24 * 4
	case "30min":
		candlesPerDay = 24 * 2
	case "45min":
		candlesPerDay = 24 * 60 / 45 // Using integer division to get proper count
	case "1h":
		candlesPerDay = 24
	case "2h":
		candlesPerDay = 12
	case "4h":
		candlesPerDay = 6
	case "8h":
		candlesPerDay = 3
	case "1day":
		candlesPerDay = 1
	case "1week":
		// For weekly candles, need to convert to a daily equivalent (approximately 1/7 of a candle per day)
		candlesPerDay = 1
		days = days / 7
		if days < 1 {
			days = 1
		}
	case "1month":
		// For monthly candles, need to convert to a daily equivalent (approximately 1/30 of a candle per day)
		candlesPerDay = 1
		days = days / 30
		if days < 1 {
			days = 1
		}
	}

	// Calculate the number of candles for the specified days and add a buffer
	return int(float64(candlesPerDay) * float64(days) * 1.1)
}
