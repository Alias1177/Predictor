package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/Alias1177/Predictor/config"
	"github.com/Alias1177/Predictor/internal/analyze"
	"github.com/Alias1177/Predictor/internal/anomaly"
	"github.com/Alias1177/Predictor/internal/baktest"
	"github.com/Alias1177/Predictor/internal/calculate"
	"github.com/Alias1177/Predictor/models"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// если .env лежит в корне проекта, без аргумента он сам найдёт
	if err := godotenv.Load(); err != nil {
		log.Warn().Msg(".env file not found, relying on actual environment variables")
	}
}

func main() {
	// 1) Настраиваем логгер и парсим конфиг
	var cfg models.Config

	// Загружаем значения из переменных окружения
	cfg.TwelveAPIKey = os.Getenv("TWELVE_API_KEY")
	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	cfg.Symbol = os.Getenv("SYMBOL")
	if cfg.Symbol == "" {
		cfg.Symbol = "EUR/USD"
	}

	// Загружаем интервал и проверяем его
	if intervalEnv := os.Getenv("INTERVAL"); intervalEnv != "" {
		cfg.Interval = intervalEnv
	} else {
		cfg.Interval = "5min"
	}

	// Загружаем числовые параметры
	if candleCountEnv := os.Getenv("CANDLE_COUNT"); candleCountEnv != "" {
		if val, err := strconv.Atoi(candleCountEnv); err == nil {
			cfg.CandleCount = val
		} else {
			cfg.CandleCount = 40
		}
	} else {
		cfg.CandleCount = 40
	}

	if rsiPeriodEnv := os.Getenv("RSI_PERIOD"); rsiPeriodEnv != "" {
		if val, err := strconv.Atoi(rsiPeriodEnv); err == nil {
			cfg.RSIPeriod = val
		} else {
			cfg.RSIPeriod = 9
		}
	} else {
		cfg.RSIPeriod = 9
	}

	if macdFastEnv := os.Getenv("MACD_FAST_PERIOD"); macdFastEnv != "" {
		if val, err := strconv.Atoi(macdFastEnv); err == nil {
			cfg.MACDFastPeriod = val
		} else {
			cfg.MACDFastPeriod = 7
		}
	} else {
		cfg.MACDFastPeriod = 7
	}

	if macdSlowEnv := os.Getenv("MACD_SLOW_PERIOD"); macdSlowEnv != "" {
		if val, err := strconv.Atoi(macdSlowEnv); err == nil {
			cfg.MACDSlowPeriod = val
		} else {
			cfg.MACDSlowPeriod = 14
		}
	} else {
		cfg.MACDSlowPeriod = 14
	}

	if macdSignalEnv := os.Getenv("MACD_SIGNAL_PERIOD"); macdSignalEnv != "" {
		if val, err := strconv.Atoi(macdSignalEnv); err == nil {
			cfg.MACDSignalPeriod = val
		} else {
			cfg.MACDSignalPeriod = 5
		}
	} else {
		cfg.MACDSignalPeriod = 5
	}

	if bbPeriodEnv := os.Getenv("BB_PERIOD"); bbPeriodEnv != "" {
		if val, err := strconv.Atoi(bbPeriodEnv); err == nil {
			cfg.BBPeriod = val
		} else {
			cfg.BBPeriod = 16
		}
	} else {
		cfg.BBPeriod = 16
	}

	if bbStdDevEnv := os.Getenv("BB_STD_DEV"); bbStdDevEnv != "" {
		if val, err := strconv.ParseFloat(bbStdDevEnv, 64); err == nil {
			cfg.BBStdDev = val
		} else {
			cfg.BBStdDev = 2.2
		}
	} else {
		cfg.BBStdDev = 2.2
	}

	if emaPeriodEnv := os.Getenv("EMA_PERIOD"); emaPeriodEnv != "" {
		if val, err := strconv.Atoi(emaPeriodEnv); err == nil {
			cfg.EMAPeriod = val
		} else {
			cfg.EMAPeriod = 10
		}
	} else {
		cfg.EMAPeriod = 10
	}

	if adxPeriodEnv := os.Getenv("ADX_PERIOD"); adxPeriodEnv != "" {
		if val, err := strconv.Atoi(adxPeriodEnv); err == nil {
			cfg.ADXPeriod = val
		} else {
			cfg.ADXPeriod = 14
		}
	} else {
		cfg.ADXPeriod = 14
	}

	if atrPeriodEnv := os.Getenv("ATR_PERIOD"); atrPeriodEnv != "" {
		if val, err := strconv.Atoi(atrPeriodEnv); err == nil {
			cfg.ATRPeriod = val
		} else {
			cfg.ATRPeriod = 14
		}
	} else {
		cfg.ATRPeriod = 14
	}

	// Обработка boolean значений
	if adaptiveEnv := os.Getenv("ADAPTIVE_INDICATOR"); adaptiveEnv != "" {
		cfg.AdaptiveIndicator = adaptiveEnv == "true" || adaptiveEnv == "1" || adaptiveEnv == "yes"
	} else {
		cfg.AdaptiveIndicator = true
	}

	if backtestEnv := os.Getenv("ENABLE_BACKTEST"); backtestEnv != "" {
		cfg.EnableBacktest = backtestEnv == "true" || backtestEnv == "1" || backtestEnv == "yes"
	} else {
		cfg.EnableBacktest = true
	}

	if backtestDaysEnv := os.Getenv("BACKTEST_DAYS"); backtestDaysEnv != "" {
		if val, err := strconv.Atoi(backtestDaysEnv); err == nil {
			cfg.BacktestDays = val
		} else {
			cfg.BacktestDays = 5
		}
	} else {
		cfg.BacktestDays = 5
	}

	// Для отладки выводим текущие значения конфигурации
	fmt.Printf("Используемая конфигурация:\n")
	fmt.Printf("Symbol: %s\n", cfg.Symbol)
	fmt.Printf("Interval: %s\n", cfg.Interval)
	fmt.Printf("CandleCount: %d\n", cfg.CandleCount)
	fmt.Printf("RSI Period: %d\n", cfg.RSIPeriod)
	fmt.Printf("MACD Fast: %d, Slow: %d, Signal: %d\n", cfg.MACDFastPeriod, cfg.MACDSlowPeriod, cfg.MACDSignalPeriod)
	fmt.Printf("BB Period: %d, StdDev: %.2f\n", cfg.BBPeriod, cfg.BBStdDev)
	fmt.Printf("EMA Period: %d\n", cfg.EMAPeriod)
	fmt.Printf("ADX Period: %d\n", cfg.ADXPeriod)
	fmt.Printf("ATR Period: %d\n", cfg.ATRPeriod)
	fmt.Printf("Adaptive Indicator: %t\n", cfg.AdaptiveIndicator)
	fmt.Printf("Backtest: %t, Days: %d\n", cfg.EnableBacktest, cfg.BacktestDays)

	lvl, _ := zerolog.ParseLevel("info")
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(lvl)

	fmt.Println("Backtest enabled:", cfg.EnableBacktest)

	// Остальной код остается без изменений
	client := config.NewClient(&cfg)
	ctx := context.Background()
	if cfg.EnableBacktest {
		log.Info().Msg("Running backtesting...")
		results, err := baktest.RunBacktest(ctx, client, &cfg)
		if err != nil {
			log.Error().Err(err).Msg("Backtest failed")
		} else if results != nil {
			// Вычисляем общую прибыль/убыток в процентах
			initialBalance := 10000.0 // Начальный баланс
			finalBalance := initialBalance
			if len(results.DetailedResults) > 0 {
				// Рассчитываем финальный баланс на основе сделок
				for _, result := range results.DetailedResults {
					if result.WasCorrect {
						// Примерная прибыль (можно настроить под ваши формулы)
						finalBalance += results.AverageGain
					} else {
						finalBalance -= results.AverageLoss
					}
				}
			}
			totalProfitPercent := ((finalBalance - initialBalance) / initialBalance) * 100

			// Выводим результаты бэктестинга с процентами
			fmt.Printf("\n===== РЕЗУЛЬТАТЫ БЭКТЕСТИНГА =====\n")
			fmt.Printf("Всего сделок: %d\n", results.TotalTrades)
			fmt.Printf("Успешных сделок: %d (%.2f%%)\n", results.WinningTrades, results.WinPercentage)
			fmt.Printf("Общая доходность: %.2f%%\n", totalProfitPercent)
			fmt.Printf("Средняя прибыль на сделку: %.2f пунктов (%.2f%%)\n",
				results.AverageGain, results.AverageGainPercent)
			fmt.Printf("Средний убыток на сделку: %.2f пунктов (%.2f%%)\n",
				results.AverageLoss, results.AverageLossPercent)

			// Коэффициент прибыли
			fmt.Printf("Коэффициент прибыли: %.2f\n", results.ProfitFactor)

			// Максимальная просадка
			fmt.Printf("Максимальная просадка: %.2f%%\n", results.MaxDrawdown)

			fmt.Printf("Макс. последовательных выигрышей: %d\n", results.MaxConsecutive.Wins)
			fmt.Printf("Макс. последовательных проигрышей: %d\n", results.MaxConsecutive.Loses)

			// Выводим производительность по режимам рынка в процентах
			fmt.Println("\nПроизводительность по режимам рынка:")
			for regime, winRate := range results.MarketRegimePerformance {
				if winRate > 0 {
					fmt.Printf("- %s: %.2f%%\n", regime, winRate)
				}
			}

			// Отображение месячной доходности
			if len(results.MonthlyReturns) > 0 {
				fmt.Println("\nДоходность по месяцам:")

				// Сортируем ключи для хронологического вывода
				months := make([]string, 0, len(results.MonthlyReturns))
				for month := range results.MonthlyReturns {
					months = append(months, month)
				}
				sort.Strings(months)

				for _, month := range months {
					returnValue := results.MonthlyReturns[month]
					sign := ""
					if returnValue > 0 {
						sign = "+"
					}
					fmt.Printf("- %s: %s%.2f%%\n", month, sign, returnValue)
				}
			}

			// Вывод для различных таймфреймов (если есть)
			if len(results.TimeframePerformance) > 0 {
				fmt.Println("\nЭффективность по таймфреймам:")
				for timeframe, performance := range results.TimeframePerformance {
					fmt.Printf("- %s: %.2f%%\n", timeframe, performance)
				}
			}

			// Общий рост капитала
			fmt.Printf("\nОбщий рост капитала: %.2f%%\n", results.EquityGrowthPercent)

			// Removed duplicate line:
			// fmt.Printf("Успешных сделок: %d (%.2f%%)\n", results.WinningTrades, results.WinPercentage)
		} else {
			log.Error().Msg("Backtest returned nil results")
		}
	}

	// 3) Загружаем свечи
	candles, err := client.GetCandles(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("fetch candles failed")
	}

	// 4) Считаем все индикаторы
	indicators := calculate.CalculateAllIndicators(candles, &cfg)

	// 5) Multi-timeframe (необязательно, если вам нужно)
	mtfData, err := calculate.GetMultiTimeframeData(ctx, cfg.TwelveAPIKey, cfg.Symbol)
	if err != nil {
		log.Warn().Err(err).Msg("mtf fetch failed")
	}

	// 6) Режим рынка и аномалии
	anomaly2 := anomaly.DetectMarketAnomalies(candles)
	regime, err := anomaly.EnhancedMarketRegimeClassification(candles)
	if err != nil {
		log.Error().Err(err).Msg("Failed to classify market regime")
		regime = &models.MarketRegime{
			Type:      "UNKNOWN",
			Strength:  0,
			Direction: "NEUTRAL",
		}
	}

	// 7) Генерируем прогноз
	prediction, err := analyze.EnhancedPrediction(
		context.Background(), candles, indicators, mtfData, regime, anomaly2, &cfg)
	if err != nil {
		log.Error().Err(err).Msg("Prediction failed")
	} else {
		fmt.Printf("Prediction: %s (conf=%s score=%.2f)\nFactors: %v\n",
			prediction.Direction, prediction.Confidence, prediction.Score, prediction.Factors)
	}

	// 8) Формируем prompt и шлём в OpenAI
	//prompt := gpt.FormatPrompt(candles, cfg.Symbol)
	//gpt.AskGPT(cfg.OpenAIAPIKey, prompt) // использует cfg.OpenAIAPIKey внутри
}
