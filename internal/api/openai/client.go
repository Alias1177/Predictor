package openai

import (
	"chi/Predictor/internal/model"
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai"
	"strings"
)

// Client wraps the OpenAI API client
type Client struct {
	client *openai.Client
	logger zerolog.Logger
}

// NewClient creates a new OpenAI client
func NewClient(apiKey string) *Client {
	return &Client{
		client: openai.NewClient(apiKey),
		logger: log.With().Str("component", "openai_client").Logger(),
	}
}

// GenerateCompletion sends a prompt to OpenAI and returns the completion
func (c *Client) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	c.logger.Debug().Str("prompt", prompt).Msg("Sending prompt to OpenAI")

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		c.logger.Error().Err(err).Msg("OpenAI API error")
		return "", err
	}

	if len(resp.Choices) == 0 {
		c.logger.Warn().Msg("OpenAI returned empty choices")
		return "", nil
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateMarketAnalysis creates a market analysis from candle data
func (c *Client) GenerateMarketAnalysis(ctx context.Context, symbol string, candles []string) (string, error) {
	// Create a prompt with the latest candle data
	prompt := "Analyze the following " + symbol + " price candles and predict the likely direction for the next 5 minutes (up or down):\n\n"

	// Add the most recent candles to the prompt
	for _, candle := range candles {
		prompt += candle + "\n"
	}

	prompt += "\nProvide your prediction in the following format:\n"
	prompt += "Direction: UP/DOWN\n"
	prompt += "Explanation: <1-2 sentences explaining your prediction>"

	return c.GenerateCompletion(ctx, prompt)
}

// FormatCandlePrompt creates a formatted prompt for OpenAI based on candle data
func FormatCandlePrompt(symbol string, candles []model.Candle) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Последние 5 свечей по %s:\n", symbol))

	start := len(candles) - 5
	if start < 0 {
		start = 0
	}

	for i := start; i < len(candles); i++ {
		c := candles[i]
		sb.WriteString(fmt.Sprintf("%s: %.5f → %.5f\n", c.Datetime, c.Open, c.Close))
	}

	sb.WriteString(`
На основе этих данных скажи, куда пойдёт график в ближайшие 5 минут, вверх или вниз.
Ответь в формате:
Направление: вверх/вниз
Пояснение: <1-2 предложения>
`)

	return sb.String()
}
