package config

import (
	"chi/Predictor/models"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
	config     *models.Config
	logger     zerolog.Logger
}

// NewClient creates a new API client with rate limiting
func NewClient(config *models.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Duration(config.RequestTimeout) * time.Second,
		},
		limiter: rate.NewLimiter(rate.Every(time.Second), 5), // 5 requests per second
		config:  config,
		logger:  log.With().Str("component", "api_client").Logger(),
	}
}

func (c *Client) GetCandles(ctx context.Context) ([]models.Candle, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	url := fmt.Sprintf(
		"https://api.twelvedata.com/time_series?symbol=%s&interval=%s&outputsize=%d&apikey=%s",
		c.config.Symbol,
		c.config.Interval,
		c.config.CandleCount,
		c.config.TwelveAPIKey,
	)

	c.logger.Debug().Str("url", url).Msg("Fetching candles")

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Use exponential backoff for retries
	var resp *http.Response
	operation := func() error {
		var err error
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("HTTP request failed: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("non-200 status code: %d", resp.StatusCode)
		}
		return nil
	}

	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.MaxElapsedTime = 30 * time.Second

	if err := backoff.Retry(operation, backoffStrategy); err != nil {
		return nil, fmt.Errorf("after retries: %w", err)
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

	var data models.TwelveResponse
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

	var candles []models.Candle
	for _, v := range data.Values {
		candles = append(candles, models.Candle{
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

func (c *Client) GetHistoricalCandles(ctx context.Context, days int) ([]models.Candle, error) {
	// For backtesting, we need more data
	// This is a simplified version - in a real implementation, you might need to handle pagination
	// and fetch data day by day if the API has limitations

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Calculate candles needed for backtest based on interval and days
	outputSize := models.CalculateCandlesForBacktest(c.config.Interval, days)

	url := fmt.Sprintf(
		"https://api.twelvedata.com/time_series?symbol=%s&interval=%s&outputsize=%d&apikey=%s",
		c.config.Symbol,
		c.config.Interval,
		outputSize,
		c.config.TwelveAPIKey,
	)

	c.logger.Debug().Str("url", url).Int("outputSize", outputSize).Msg("Fetching historical candles for backtesting")

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Use exponential backoff for retries
	var resp *http.Response
	operation := func() error {
		var err error
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("HTTP request failed: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("non-200 status code: %d", resp.StatusCode)
		}
		return nil
	}

	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.MaxElapsedTime = 30 * time.Second

	if err := backoff.Retry(operation, backoffStrategy); err != nil {
		return nil, fmt.Errorf("after retries: %w", err)
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

	var data models.TwelveResponse
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

	var candles []models.Candle
	for _, v := range data.Values {
		candles = append(candles, models.Candle{
			Datetime: v.Datetime,
			Open:     v.Open,
			High:     v.High,
			Low:      v.Low,
			Close:    v.Close,
			Volume:   v.Volume,
		})
	}

	c.logger.Debug().Int("count", len(candles)).Msg("Fetched historical candles")
	return candles, nil
}
