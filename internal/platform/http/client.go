package http

import (
	"context"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/time/rate"
)

// Client is a wrapper for HTTP client with rate limiting
type Client struct {
	HTTPClient *http.Client
	Limiter    *rate.Limiter
}

// ClientOptions holds options for creating a new Client
type ClientOptions struct {
	Timeout         time.Duration
	RequestsPerSec  int
	MaxRetries      int
	MaxRetryTimeout time.Duration
}

// NewClient creates a new HTTP client with rate limiting
func NewClient(opts ClientOptions) *Client {
	// Set default values if not provided
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.RequestsPerSec == 0 {
		opts.RequestsPerSec = 5
	}
	if opts.MaxRetryTimeout == 0 {
		opts.MaxRetryTimeout = 30 * time.Second
	}

	return &Client{
		HTTPClient: &http.Client{
			Timeout: opts.Timeout,
		},
		Limiter: rate.NewLimiter(rate.Every(time.Second), opts.RequestsPerSec),
	}
}

// DoRequest performs an HTTP request with rate limiting and retries
func (c *Client) DoRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Wait for rate limiter
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Use exponential backoff for retries
	var resp *http.Response
	operation := func() error {
		var err error
		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return &HTTPStatusError{StatusCode: resp.StatusCode}
		}
		return nil
	}

	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.MaxElapsedTime = 30 * time.Second

	if err := backoff.Retry(operation, backoffStrategy); err != nil {
		return nil, err
	}

	return resp, nil
}

// HTTPStatusError represents an error due to a non-200 HTTP status code
type HTTPStatusError struct {
	StatusCode int
}

// Error implements the error interface
func (e *HTTPStatusError) Error() string {
	return "non-200 status code: " + http.StatusText(e.StatusCode)
}
