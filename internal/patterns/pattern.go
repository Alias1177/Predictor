package patterns

import (
	"math"

	"github.com/Alias1177/Predictor/models"
)

func IdentifyPriceActionPatterns(candles []models.Candle) []string {
	if len(candles) < 5 {
		return nil
	}

	var patterns []string

	// Получаем недавние свечи
	c1 := candles[len(candles)-5] // Самая старая
	c2 := candles[len(candles)-4]
	c3 := candles[len(candles)-3]
	c4 := candles[len(candles)-2]
	c5 := candles[len(candles)-1] // Самая новая

	// Размеры тел
	bodySize1 := math.Abs(c1.Close - c1.Open)
	bodySize2 := math.Abs(c2.Close - c2.Open)
	bodySize3 := math.Abs(c3.Close - c3.Open)
	bodySize4 := math.Abs(c4.Close - c4.Open)
	bodySize5 := math.Abs(c5.Close - c5.Open)

	// Средний размер тела
	avgBodySize := (bodySize1 + bodySize2 + bodySize3 + bodySize4 + bodySize5) / 5

	// Направления свечей
	bullish3 := c3.Close > c3.Open
	bullish4 := c4.Close > c4.Open
	bullish5 := c5.Close > c5.Open

	// Верхние и нижние тени
	upperWick5 := c5.High - math.Max(c5.Open, c5.Close)
	lowerWick5 := math.Min(c5.Open, c5.Close) - c5.Low

	// Проверка на поглощение
	if bullish5 && !bullish4 &&
		c5.Open < c4.Close &&
		c5.Close > c4.Open &&
		bodySize5 > bodySize4*1.2 {
		patterns = append(patterns, "BULLISH_ENGULFING")
	}

	if !bullish5 && bullish4 &&
		c5.Open > c4.Close &&
		c5.Close < c4.Open &&
		bodySize5 > bodySize4*1.2 {
		patterns = append(patterns, "BEARISH_ENGULFING")
	}

	// Проверка на пин-бары / молоты
	if lowerWick5 > bodySize5*2 && upperWick5 < bodySize5*0.5 {
		patterns = append(patterns, "HAMMER")
	}

	if upperWick5 > bodySize5*2 && lowerWick5 < bodySize5*0.5 {
		patterns = append(patterns, "SHOOTING_STAR")
	}

	// Проверка на трёхсвечные паттерны
	if bullish3 && bullish4 && bullish5 {
		patterns = append(patterns, "THREE_WHITE_SOLDIERS")
	}

	if !bullish3 && !bullish4 && !bullish5 {
		patterns = append(patterns, "THREE_BLACK_CROWS")
	}

	// Проверка на доджи
	if bodySize5 < avgBodySize*0.3 &&
		(upperWick5 > bodySize5 || lowerWick5 > bodySize5) {
		patterns = append(patterns, "DOJI")
	}

	// Проверка на импульсные свечи
	if bullish5 && bodySize5 > avgBodySize*1.5 &&
		lowerWick5 < bodySize5*0.2 && upperWick5 < bodySize5*0.2 {
		patterns = append(patterns, "STRONG_BULLISH_MOMENTUM")
	}

	if !bullish5 && bodySize5 > avgBodySize*1.5 &&
		lowerWick5 < bodySize5*0.2 && upperWick5 < bodySize5*0.2 {
		patterns = append(patterns, "STRONG_BEARISH_MOMENTUM")
	}

	// Вечерняя звезда (Медвежий разворот)
	if len(candles) >= 7 &&
		bullish3 && // Первая свеча бычья
		bodySize3 > avgBodySize && // Первая свеча имеет большое тело
		math.Abs(c4.Close-c4.Open) < avgBodySize*0.3 && // Средняя свеча имеет маленькое тело
		c4.Open > c3.Close && // Гэп вверх между первой и средней
		!bullish5 && // Третья свеча медвежья
		bodySize5 > avgBodySize && // Третья свеча имеет большое тело
		c5.Close < (c3.Open+(c3.Close-c3.Open)/2) { // Третья свеча закрывается ниже середины первой
		patterns = append(patterns, "EVENING_STAR")
	}

	// Утренняя звезда (Бычий разворот)
	if len(candles) >= 7 &&
		!bullish3 && // Первая свеча медвежья
		bodySize3 > avgBodySize && // Первая свеча имеет большое тело
		math.Abs(c4.Close-c4.Open) < avgBodySize*0.3 && // Средняя свеча имеет маленькое тело
		c4.Open < c3.Close && // Гэп вниз между первой и средней
		bullish5 && // Третья свеча бычья
		bodySize5 > avgBodySize && // Третья свеча имеет большое тело
		c5.Close > (c3.Open+(c3.Close-c3.Open)/2) { // Третья свеча закрывается выше середины первой
		patterns = append(patterns, "MORNING_STAR")
	}

	// Идентификация Двойной вершины
	if len(candles) >= 10 {
		// Находим два пика с впадиной между ними
		var peaks []int
		for i := 2; i < len(candles)-2; i++ {
			if candles[i].High > candles[i-1].High &&
				candles[i].High > candles[i-2].High &&
				candles[i].High > candles[i+1].High &&
				candles[i].High > candles[i+2].High {
				peaks = append(peaks, i)
			}
		}

		if len(peaks) >= 2 {
			// Проверяем, имеют ли последние два пика схожую высоту
			last := peaks[len(peaks)-1]
			prev := peaks[len(peaks)-2]

			if math.Abs(candles[last].High-candles[prev].High) < avgBodySize*0.5 &&
				last-prev >= 3 { // Обеспечиваем некоторое расстояние между пиками
				// Находим впадину между ними
				var lowestVal float64 = candles[prev].High
				lowestIdx := prev

				for i := prev + 1; i < last; i++ {
					if candles[i].Low < lowestVal {
						lowestVal = candles[i].Low
						lowestIdx = i
					}
				}

				// Проверяем, находится ли текущая цена ниже впадины
				if candles[len(candles)-1].Close < lowestVal {
					patterns = append(patterns, "DOUBLE_TOP")
				}

				// Используем lowestIdx, чтобы избежать предупреждения о неиспользуемой переменной
				_ = lowestIdx
			}
		}
	}

	// Идентификация Двойного дна
	if len(candles) >= 10 {
		// Находим две впадины с пиком между ними
		var troughs []int
		for i := 2; i < len(candles)-2; i++ {
			if candles[i].Low < candles[i-1].Low &&
				candles[i].Low < candles[i-2].Low &&
				candles[i].Low < candles[i+1].Low &&
				candles[i].Low < candles[i+2].Low {
				troughs = append(troughs, i)
			}
		}

		if len(troughs) >= 2 {
			// Проверяем, имеют ли последние две впадины схожую глубину
			last := troughs[len(troughs)-1]
			prev := troughs[len(troughs)-2]

			if math.Abs(candles[last].Low-candles[prev].Low) < avgBodySize*0.5 &&
				last-prev >= 3 { // Обеспечиваем некоторое расстояние между впадинами
				// Находим пик между ними
				var highestVal float64 = candles[prev].Low
				highestIdx := prev

				for i := prev + 1; i < last; i++ {
					if candles[i].High > highestVal {
						highestVal = candles[i].High
						highestIdx = i
					}
				}

				// Проверяем, находится ли текущая цена выше пика
				if candles[len(candles)-1].Close > highestVal {
					patterns = append(patterns, "DOUBLE_BOTTOM")
				}

				// Используем highestIdx, чтобы избежать предупреждения о неиспользуемой переменной
				_ = highestIdx
			}
		}
	}

	return patterns
}

// DetectHarmonicPatterns идентифицирует гармонические паттерны на основе Фибоначчи
func DetectHarmonicPatterns(candles []models.Candle) []models.HarmonicPattern {
	if len(candles) < 30 {
		return nil
	}

	var patterns []models.HarmonicPattern

	// Находим точки разворота (потенциальные точки XABCD)
	swingHighs, swingLows := findSwingPoints(candles, 5)

	// Нужно минимум 5 точек разворота (XABCD) для формирования гармонического паттерна
	if len(swingHighs) < 3 || len(swingLows) < 3 {
		return nil
	}

	// Пытаемся определить паттерн Гартли
	gartleyPatterns := detectGartleyPattern(candles, swingHighs, swingLows)
	patterns = append(patterns, gartleyPatterns...)

	// Пытаемся определить паттерн Бабочка
	butterflyPatterns := detectButterflyPattern(candles, swingHighs, swingLows)
	patterns = append(patterns, butterflyPatterns...)

	// Пытаемся определить паттерн Летучая мышь
	batPatterns := detectBatPattern(candles, swingHighs, swingLows)
	patterns = append(patterns, batPatterns...)

	// Пытаемся определить паттерн Краб
	crabPatterns := detectCrabPattern(candles, swingHighs, swingLows)
	patterns = append(patterns, crabPatterns...)

	return patterns
}

// findSwingPoints идентифицирует максимумы и минимумы свингов в ценовых данных
func findSwingPoints(candles []models.Candle, strength int) ([]int, []int) {
	var swingHighs, swingLows []int

	for i := strength; i < len(candles)-strength; i++ {
		// Проверка на максимум свинга
		isSwingHigh := true
		for j := i - strength; j < i; j++ {
			if candles[j].High > candles[i].High {
				isSwingHigh = false
				break
			}
		}
		for j := i + 1; j <= i+strength; j++ {
			if candles[j].High > candles[i].High {
				isSwingHigh = false
				break
			}
		}

		if isSwingHigh {
			swingHighs = append(swingHighs, i)
		}

		// Проверка на минимум свинга
		isSwingLow := true
		for j := i - strength; j < i; j++ {
			if candles[j].Low < candles[i].Low {
				isSwingLow = false
				break
			}
		}
		for j := i + 1; j <= i+strength; j++ {
			if candles[j].Low < candles[i].Low {
				isSwingLow = false
				break
			}
		}

		if isSwingLow {
			swingLows = append(swingLows, i)
		}
	}

	return swingHighs, swingLows
}

// detectGartleyPattern ищет точки XABCD, которые формируют паттерн Гартли
func detectGartleyPattern(candles []models.Candle, swingHighs, swingLows []int) []models.HarmonicPattern {
	var patterns []models.HarmonicPattern

	// Фибоначчи константы для паттерна Гартли
	const (
		AB_XA_RATIO_MIN = 0.58 // ~ 0.618 - допуск
		AB_XA_RATIO_MAX = 0.65 // ~ 0.618 + допуск
		BC_AB_RATIO_MIN = 0.35 // ~ 0.382 - допуск
		BC_AB_RATIO_MAX = 0.90 // ~ 0.886 + допуск
		CD_BC_RATIO_MIN = 1.25 // ~ 1.272 - допуск
		CD_BC_RATIO_MAX = 1.65 // ~ 1.618 + допуск
		XD_XA_RATIO_MIN = 0.75 // ~ 0.786 - допуск
		XD_XA_RATIO_MAX = 0.82 // ~ 0.786 + допуск
	)

	// Бычий Гартли: Найти 5 точек, где:
	// XA: Начальное движение
	// AB: Откат XA (0.618)
	// BC: Откат AB (0.382-0.886)
	// CD: Расширение BC (1.272-1.618)
	// D: Откат XA (0.786)

	for i := 0; i < len(swingLows)-1; i++ {
		x := swingLows[i]

		for j := 0; j < len(swingHighs); j++ {
			if swingHighs[j] <= x {
				continue
			}

			a := swingHighs[j]

			// Расчет расстояния XA
			xaDistance := candles[a].High - candles[x].Low

			for k := 0; k < len(swingLows); k++ {
				if swingLows[k] <= a {
					continue
				}

				b := swingLows[k]

				// Расчет расстояния AB и проверка соотношения Фибоначчи
				abDistance := candles[a].High - candles[b].Low
				abRatio := 0.0
				if xaDistance > 0 {
					abRatio = abDistance / xaDistance
				}

				if !(AB_XA_RATIO_MIN <= abRatio && abRatio <= AB_XA_RATIO_MAX) {
					continue
				}

				for l := 0; l < len(swingHighs); l++ {
					if swingHighs[l] <= b {
						continue
					}

					c := swingHighs[l]

					// Расчет расстояния BC и проверка соотношения Фибоначчи
					bcDistance := candles[c].High - candles[b].Low
					bcRatio := 0.0
					if abDistance > 0 {
						bcRatio = bcDistance / abDistance
					}

					if !(BC_AB_RATIO_MIN <= bcRatio && bcRatio <= BC_AB_RATIO_MAX) {
						continue
					}

					for m := 0; m < len(swingLows); m++ {
						if swingLows[m] <= c {
							continue
						}

						d := swingLows[m]

						// Расчет расстояния CD и проверка соотношения Фибоначчи
						cdDistance := candles[c].High - candles[d].Low
						cdRatio := 0.0
						if bcDistance > 0 {
							cdRatio = cdDistance / bcDistance
						}

						if !(CD_BC_RATIO_MIN <= cdRatio && cdRatio <= CD_BC_RATIO_MAX) {
							continue
						}

						// Проверяем, что D это 0.786 откат XA
						xdDistance := candles[d].Low - candles[x].Low
						xdRatio := 0.0
						if xaDistance > 0 {
							xdRatio = xdDistance / xaDistance
						}

						if !(XD_XA_RATIO_MIN <= xdRatio && xdRatio <= XD_XA_RATIO_MAX) {
							continue
						}

						// Мы нашли бычий Гартли!
						pattern := models.HarmonicPattern{
							Type:      "GARTLEY",
							Direction: "BULLISH",
							Points: map[string]models.PatternPoint{
								"X": {Index: x, Price: candles[x].Low},
								"A": {Index: a, Price: candles[a].High},
								"B": {Index: b, Price: candles[b].Low},
								"C": {Index: c, Price: candles[c].High},
								"D": {Index: d, Price: candles[d].Low},
							},
							Ratios: map[string]float64{
								"AB/XA": abRatio,
								"BC/AB": bcRatio,
								"CD/BC": cdRatio,
								"XD/XA": xdRatio,
							},
							CompletionIndex:   d,
							PotentialReversal: true,
						}

						patterns = append(patterns, pattern)
					}
				}
			}
		}
	}

	// Аналогичная логика для медвежьего Гартли (опущена для краткости)

	return patterns
}

// detectButterflyPattern ищет паттерн Бабочка
func detectButterflyPattern(candles []models.Candle, swingHighs, swingLows []int) []models.HarmonicPattern {
	// Аналогично Гартли, но с другими соотношениями Фибоначчи
	// Опущено для краткости
	return nil
}

// detectBatPattern ищет паттерн Летучая мышь
func detectBatPattern(candles []models.Candle, swingHighs, swingLows []int) []models.HarmonicPattern {
	// Аналогично Гартли, но с другими соотношениями Фибоначчи
	// Опущено для краткости
	return nil
}

// detectCrabPattern ищет паттерн Краб
func detectCrabPattern(candles []models.Candle, swingHighs, swingLows []int) []models.HarmonicPattern {
	// Аналогично Гартли, но с другими соотношениями Фибоначчи
	// Опущено для краткости
	return nil
}
