package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/Alias1177/Predictor/internal/database"
	"github.com/Alias1177/Predictor/internal/payment"

	_ "github.com/lib/pq" // PostgreSQL драйвер

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Initialize database with PostgreSQL connection
	dbParams := database.ConnectionParams{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}

	log.Printf("Webhook server starting with DB params: host=%s, port=%s, user=%s, dbname=%s",
		dbParams.Host, dbParams.Port, dbParams.User, dbParams.DBName)

	db, err := database.New(dbParams)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Stripe service
	stripeService := payment.NewStripeService()
	log.Printf("Stripe initialized. Webhook secret: %s (length: %d)",
		maskSecret(os.Getenv("STRIPE_WEBHOOK_SECRET")), len(os.Getenv("STRIPE_WEBHOOK_SECRET")))

	// Set up webhook handler
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received webhook request from %s", r.RemoteAddr)

		if r.Method != "POST" {
			log.Printf("Invalid method: %s", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}

		// Log the headers for debugging
		log.Printf("Request headers: %v", r.Header)

		// Get the signature header
		signature := r.Header.Get("Stripe-Signature")
		if signature == "" {
			log.Printf("Stripe-Signature header not found")
			http.Error(w, "Stripe-Signature header required", http.StatusBadRequest)
			return
		}

		log.Printf("Stripe-Signature: %s", maskSecret(signature))

		// Verify webhook signature and parse the event
		event, err := stripeService.VerifyWebhookSignature(body, signature)
		if err != nil {
			log.Printf("Failed to verify webhook signature: %v", err)
			http.Error(w, "Invalid signature", http.StatusBadRequest)
			return
		}

		log.Printf("Webhook event verified. Event type: %s, Event ID: %s", event.Type, event.ID)

		// Log the raw payload for debugging
		log.Printf("Raw event data: %s", string(body))

		// Process the event
		userID, status, err := stripeService.ProcessSubscriptionPayment(event)
		if err != nil {
			log.Printf("Failed to process payment event: %v", err)
			http.Error(w, "Error processing event", http.StatusInternalServerError)
			return
		}

		log.Printf("Event processed. UserID: %d, Status: %s", userID, status)

		// If we have a valid user ID and status, update the subscription
		if userID > 0 {
			paymentID := event.ID
			if err := db.UpdateSubscriptionStatus(userID, status, paymentID); err != nil {
				log.Printf("Failed to update subscription status: %v", err)
				http.Error(w, "Error updating subscription", http.StatusInternalServerError)
				return
			}
			log.Printf("Successfully updated subscription for user %d to status %s with payment ID %s",
				userID, status, paymentID)

			// Verify the update was successful
			sub, err := db.GetSubscription(userID)
			if err != nil {
				log.Printf("Failed to verify subscription update: %v", err)
			} else if sub != nil {
				log.Printf("Verified subscription: UserID=%d, Status=%s, PaymentID=%s",
					sub.UserID, sub.Status, sub.PaymentID)
			} else {
				log.Printf("Warning: Could not find subscription after update for user %d", userID)
			}
		} else {
			log.Printf("Warning: No valid userID found in event")
		}

		// Return a success response to Stripe
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		log.Printf("Webhook processed successfully")
	})

	// Add a simple health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Webhook server is running"))
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting webhook server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// maskSecret masks a secret string for logging (shows first 3 and last 3 characters)
func maskSecret(secret string) string {
	if len(secret) < 7 {
		return "***"
	}
	return secret[:3] + "..." + secret[len(secret)-3:]
}
