package gpt

//
//import (
//	"github.com/Alias1177/Predictor/models"
//	"context"
//	"fmt"
//	"github.com/rs/zerolog/log"
//	"github.com/sashabaranov/go-openai"
//	"strings"
//)
//
//func FormatPrompt(candles []models.Candle, symbol string) string {
//	var sb strings.Builder
//	sb.WriteString(fmt.Sprintf("Последние 5 свечей по %s:\n", symbol))
//	start := len(candles) - 15
//	if start < 0 {
//		start = 0
//	}
//	for i := start; i < len(candles); i++ {
//		c := candles[i]
//		sb.WriteString(fmt.Sprintf("%s: %.5f → %.5f\n", c.Datetime, c.Open, c.Close))
//	}
//	sb.WriteString(`
//На основе этих данных скажи, куда пойдёт график в ближайшие 5 минут, вверх или вниз.
//Ответь в формате:
//Направление: вверх/вниз
//Пояснение: <1-2 предложения>
//`)
//	return sb.String()
//}
//
//// askGPT шлёт prompt в OpenAI и печатает ответ
//func AskGPT(openaiKey, prompt string) {
//	client := openai.NewClient(openaiKey)
//	resp, err := client.CreateChatCompletion(
//		context.Background(),
//		openai.ChatCompletionRequest{
//			Model: openai.GPT4,
//			Messages: []openai.ChatCompletionMessage{
//				{Role: openai.ChatMessageRoleUser, Content: prompt},
//			},
//		},
//	)
//	if err != nil {
//		log.Error().Err(err).Msg("OpenAI error")
//		return
//	}
//	fmt.Println("📈 Ответ модели:\n", resp.Choices[0].Message.Content)
//}
//func ProcessGPT(ctx context.Context, openaiKey, prompt string) (string, error) {
//	client := openai.NewClient(openaiKey)
//	resp, err := client.CreateChatCompletion(
//		ctx,
//		openai.ChatCompletionRequest{
//			Model: openai.GPT4,
//			Messages: []openai.ChatCompletionMessage{
//				{Role: openai.ChatMessageRoleUser, Content: prompt},
//			},
//		},
//	)
//	if err != nil {
//		log.Error().Err(err).Msg("OpenAI error")
//		return "", err
//	}
//
//	return resp.Choices[0].Message.Content, nil
//}
