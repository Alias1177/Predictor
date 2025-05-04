package twelvedata

import (
	"chi/Predictor/internal/model"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client is the TwelveData API client
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     zerolog.Logger
}

// ClientOptions holds options for creating a new TwelveData client
type ClientOptions struct {
	APIKey          string
	RequestTimeout  time.Duration
	RequestsPerSec  int
	MaxRetries      int
	MaxRetryTimeout time.Duration
}

// NewClient creates a new TwelveData API client
func NewClient(options ClientOptions) *Client {
	httpOpts := httpClient.ClientOptions{
		Timeout:         options.RequestTimeout,
		RequestsPerSec:  options.RequestsPerSec,
		MaxRetries:      options.MaxRetries,
		MaxRetryTimeout: options.MaxRetryTimeout,
	}

	// Apply defaults if not set
	if httpOpts.Timeout == 0 {
		httpOpts.Timeout = 30 * time.Second
	}
	if httpOpts.RequestsPerSec == 0 {
		httpOpts.RequestsPerSec = 5
	}

	return &Client{
		apiKey:     options.APIKey,
		baseURL:    "https://api.twelvedata.com",
		httpClient: httpClient.NewClient(httpOpts),
		logger:     log.With().Str("component", "twelvedata_client").Logger(),
	}
}

// GetCandles fetches candle data from Twelve Data API
func (c *Client) GetCandles(ctx context.Context, symbol string, interval string, count int) ([]model.Candle, error) {
	url := fmt.Sprintf(
		"%s/time_series?symbol=%s&interval=%s&outputsize=%d&apikey=%s",
		c.baseURL,
		symbol,
		interval,
		count,
		c.apiKey,
	)

	c.logger.Debug().Str("url", url).Msg("Fetching candles")

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.DoRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if strings.Contains(string(body), `"status":"error"`) {
		c.logger.Error().Str("response", string(body)).Msg("Twelve Data API error")
		return nil, fmt.Errorf("Twelve Data API error: %s", string(body))
	}

	var data model.TwelveResponse
	if err := json.Unmarshal(body, &data); err != nil {
		c.logger.Error().Err(err).Str("response", string(body)).Msg("Error parsing JSON")
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	if len(data.Values) == 0 {
		c.logger.Warn().Str("response", string(body)).Msg("No candles in response")
		return nil, fmt.Errorf("empty data returned")
	}

	// Sort candles by datetime (oldest first for proper calculations)
	sort.Slice(data.Values, func(i, j int) bool {
		return data.Values[i].Datetime < data.Values[j].Datetime
	})

	var candles []model.Candle
	for _, v := range data.Values {
		candles = append(candles, model.Candle{
			Datetime: v.Datetime,
			Open:     v.Open,
			High:     v.High,
			Low:      v.Low,
			Close:    v.Close,
			Volume:   v.Volume,
		})
	}

	c.logger.Debug().Int("count", len(candles)).Msg("Fetched candles")
	return candles, nil
}

// GetHistoricalCandles fetches historical candle data for backtesting
func (c *Client) GetHistoricalCandles(ctx context.Context, symbol string, interval string, days int) ([]model.Candle, error) {
	// For backtesting, we need more data
	count := calculateCandlesForBacktest(interval, days)

	return c.GetCandles(ctx, symbol, interval, count)
}

// calculateCandlesForBacktest estimates how many candles are needed for backtesting
func calculateCandlesForBacktest(interval string, days int) int {
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
