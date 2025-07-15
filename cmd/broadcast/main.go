package main

import (
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/Alias1177/Predictor/internal/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, relying on actual environment variables")
	}
}

func main() {
	// Initialize database
	dbParams := database.ConnectionParams{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}

	db, err := database.New(dbParams)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Telegram bot
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set in environment")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	// Get all users from database
	users, err := db.GetAllUsers()
	if err != nil {
		log.Fatalf("Failed to get users from database: %v", err)
	}

	log.Printf("Found %d users in database", len(users))

	// Broadcast message
	message := "🔥 **IMPORTANT ANNOUNCEMENT** 🔥\n\n" +
		"📢 **Special 24-Hour Pricing!**\n\n" +
		"For the next 24 hours only, our premium trading bot will be available for just **$2.9**!\n\n" +
		"🚀 This is a limited-time offer to get access to:\n" +
		"• Advanced market predictions\n" +
		"• Multiple trading pairs\n" +
		"• Real-time technical analysis\n" +
		"• Professional trading signals\n\n" +
		"⏰ **Don't miss out!** This special price expires in 24 hours.\n\n" +
		"💰 Upgrade now and start making smarter trading decisions!\n\n" +
		"Use the /start command to begin!"

	successCount := 0
	errorCount := 0

	for i, user := range users {
		// Send message to user
		msg := tgbotapi.NewMessage(user.ChatID, message)
		msg.ParseMode = "Markdown"

		_, err := bot.Send(msg)
		if err != nil {
			log.Printf("Failed to send message to user %d (chat_id: %d): %v",
				user.UserID, user.ChatID, err)
			errorCount++
		} else {
			log.Printf("✅ Message sent to user %d (chat_id: %d) [%d/%d]",
				user.UserID, user.ChatID, i+1, len(users))
			successCount++
		}

		// Add delay between messages to avoid rate limiting
		// Telegram allows 30 messages per second for bots, so we use 50ms delay
		if i < len(users)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Final statistics
	log.Printf("\n=== BROADCAST COMPLETED ===")
	log.Printf("Total users: %d", len(users))
	log.Printf("Successfully sent: %d", successCount)
	log.Printf("Failed to send: %d", errorCount)
	log.Printf("Success rate: %.2f%%", float64(successCount)/float64(len(users))*100)

	fmt.Printf("\n🎯 Broadcast completed!\n")
	fmt.Printf("📊 Stats: %d sent, %d failed out of %d total users\n",
		successCount, errorCount, len(users))
}
