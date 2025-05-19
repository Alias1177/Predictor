package indicators

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Alias1177/Predictor/models"
)

type IndicatorCache struct {
	mu         sync.RWMutex
	rsi        map[string]float64
	ema        map[string]float64
	volatility map[string]float64
	lastUpdate map[string]time.Time
}

var cache = &IndicatorCache{
	rsi:        make(map[string]float64),
	ema:        make(map[string]float64),
	volatility: make(map[string]float64),
	lastUpdate: make(map[string]time.Time),
}

const cacheTTL = 5 * time.Minute

// Очистка устаревших данных из кэша
func cleanupCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := time.Now()
	for key, lastUpdate := range cache.lastUpdate {
		if now.Sub(lastUpdate) > cacheTTL {
			delete(cache.rsi, key)
			delete(cache.ema, key)
			delete(cache.volatility, key)
			delete(cache.lastUpdate, key)
		}
	}
}

// Запускаем очистку кэша каждые 5 минут
func init() {
	go func() {
		for {
			time.Sleep(cacheTTL)
			cleanupCache()
		}
	}()
}

func getCacheKey(candles []models.Candle, indicator string, period int) string {
	if len(candles) == 0 {
		return ""
	}
	lastCandle := candles[len(candles)-1]
	return fmt.Sprintf("%s_%d_%s", indicator, period, lastCandle.Timestamp.Format("2006-01-02 15:04:05"))
}

// CalculateRSI рассчитывает индекс относительной силы
func CalculateRSI(candles []models.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50.0 // Возвращаем нейтральное значение при недостатке данных
	}

	cacheKey := getCacheKey(candles, "RSI", period)

	cache.mu.RLock()
	if value, ok := cache.rsi[cacheKey]; ok {
		if time.Since(cache.lastUpdate[cacheKey]) < cacheTTL {
			cache.mu.RUnlock()
			return value
		}
	}
	cache.mu.RUnlock()

	var gains, losses float64
	for i := 1; i <= period; i++ {
		change := candles[len(candles)-i].Close - candles[len(candles)-i-1].Close
		if change >= 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	if losses == 0 {
		return 100.0
	}

	rs := gains / losses
	rsi := 100.0 - (100.0 / (1.0 + rs))

	cache.mu.Lock()
	cache.rsi[cacheKey] = rsi
	cache.lastUpdate[cacheKey] = time.Now()
	cache.mu.Unlock()

	return rsi
}

// CalculateEMA рассчитывает экспоненциальную скользящую среднюю
func CalculateEMA(candles []models.Candle, period int) float64 {
	if len(candles) < period {
		return candles[len(candles)-1].Close
	}

	multiplier := 2.0 / float64(period+1)
	ema := candles[0].Close

	for i := 1; i < len(candles); i++ {
		ema = (candles[i].Close-ema)*multiplier + ema
	}

	return ema
}

// CalculateVolatility рассчитывает волатильность на основе свечей
func CalculateVolatility(candles []models.Candle) float64 {
	if len(candles) < 20 {
		return 0
	}

	returns := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		returns[i-1] = (candles[i].Close - candles[i-1].Close) / candles[i-1].Close
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns))

	return math.Sqrt(variance)
}
