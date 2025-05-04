package models

import "context"

type CandleClient interface {
	GetCandles(ctx context.Context) ([]Candle, error)
	GetHistoricalCandles(ctx context.Context, days int) ([]Candle, error)
}
