package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Alias1177/Predictor/internal/database"
	"github.com/Alias1177/Predictor/internal/payment"

	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// TLS конфигурация
	useHTTPS := os.Getenv("USE_HTTPS") == "true"
	domain := os.Getenv("DOMAIN") // Например: yourdomain.com
	certDir := os.Getenv("CERT_DIR")
	if certDir == "" {
		certDir = "./certs"
	}

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

	// Set up routes
	mux := http.NewServeMux()

	// Set up webhook handler
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received webhook request from %s, Event-ID: %s",
			r.RemoteAddr, r.Header.Get("Stripe-Event-Id"))

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

		// Проверяем, не обработали ли мы уже это событие
		processed, err := db.IsEventProcessed(event.ID)
		if err != nil {
			log.Printf("Error checking event processing status: %v", err)
		} else if processed {
			log.Printf("Event %s already processed, skipping", event.ID)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "already_processed"})
			return
		}

		// Log the raw payload for debugging
		log.Printf("Raw event data: %s", string(body))

		// Process the event
		userID, status, subscriptionID, err := stripeService.ProcessSubscriptionPayment(event)
		if err != nil {
			log.Printf("Failed to process payment event: %v", err)
			http.Error(w, "Error processing event", http.StatusInternalServerError)
			return
		}

		log.Printf("Event processed. UserID: %d, Status: %s, SubscriptionID: %s", userID, status, subscriptionID)

		// If we have a valid user ID and status, update the subscription
		if userID > 0 {
			paymentID := event.ID

			// Если статус accepted, используем ActivateSubscription для правильной даты истечения
			if status == "accepted" {
				if err := db.ActivateSubscription(userID, paymentID, subscriptionID); err != nil {
					log.Printf("Failed to activate subscription: %v", err)
					http.Error(w, "Error activating subscription", http.StatusInternalServerError)
					return
				}
				log.Printf("Successfully activated subscription for user %d with payment ID %s", userID, paymentID)
			} else {
				// Для других статусов используем старую функцию
				if err := db.UpdateSubscriptionStatus(userID, status, paymentID); err != nil {
					log.Printf("Failed to update subscription status: %v", err)
					http.Error(w, "Error updating subscription", http.StatusInternalServerError)
					return
				}
				log.Printf("Successfully updated subscription for user %d to status %s with payment ID %s",
					userID, status, paymentID)
			}

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
		} else if subscriptionID != "" {
			// Обрабатываем случай customer.subscription.created - только обновляем subscription ID
			log.Printf("Processing subscription creation event with subscription ID: %s", subscriptionID)
		} else {
			log.Printf("Warning: No valid userID found in event")
		}

		// Отмечаем событие как обработанное
		if err := db.MarkEventProcessed(event.ID); err != nil {
			log.Printf("Error marking event as processed: %v", err)
		}

		// Return a success response to Stripe
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		log.Printf("Webhook processed successfully")
	})

	// Add a simple health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Webhook server is running"))
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	if useHTTPS {
		if domain != "" {
			// Автоматические Let's Encrypt сертификаты
			log.Printf("Starting HTTPS server with Let's Encrypt for domain: %s", domain)

			// Создаём директорию для кеша сертификатов
			if err := os.MkdirAll(certDir, 0700); err != nil {
				log.Fatalf("Failed to create cert directory: %v", err)
			}

			certManager := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(domain),
				Cache:      autocert.DirCache(certDir),
			}

			server.TLSConfig = &tls.Config{
				GetCertificate: certManager.GetCertificate,
			}

			// Запускаем HTTP сервер для ACME challenge на порту 80
			go func() {
				httpServer := &http.Server{
					Addr:    ":80",
					Handler: certManager.HTTPHandler(nil),
				}
				log.Printf("Starting HTTP server on port 80 for ACME challenge")
				if err := httpServer.ListenAndServe(); err != nil {
					log.Printf("HTTP server error: %v", err)
				}
			}()

			// Запускаем HTTPS сервер
			server.Addr = ":443"
			log.Printf("Starting HTTPS server on port 443")
			if err := server.ListenAndServeTLS("", ""); err != nil {
				log.Fatalf("Failed to start HTTPS server: %v", err)
			}
		} else {
			// Используем самоподписанные сертификаты или существующие
			certFile := os.Getenv("TLS_CERT_FILE")
			keyFile := os.Getenv("TLS_KEY_FILE")

			if certFile == "" {
				certFile = filepath.Join(certDir, "server.crt")
			}
			if keyFile == "" {
				keyFile = filepath.Join(certDir, "server.key")
			}

			// Проверяем существование сертификатов
			if _, err := os.Stat(certFile); os.IsNotExist(err) {
				log.Printf("Certificate file not found: %s", certFile)
				log.Printf("Generating self-signed certificate...")
				if err := generateSelfSignedCert(certFile, keyFile); err != nil {
					log.Fatalf("Failed to generate self-signed certificate: %v", err)
				}
			}

			log.Printf("Starting HTTPS server on port %s with certificates: %s, %s", port, certFile, keyFile)
			if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
				log.Fatalf("Failed to start HTTPS server: %v", err)
			}
		}
	} else {
		log.Printf("Starting HTTP server on port %s", port)
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}
}

// maskSecret masks a secret string for logging (shows first 3 and last 3 characters)
func maskSecret(secret string) string {
	if len(secret) < 7 {
		return "***"
	}
	return secret[:3] + "..." + secret[len(secret)-3:]
}

// generateSelfSignedCert генерирует самоподписанный сертификат
func generateSelfSignedCert(certFile, keyFile string) error {
	log.Printf("Generating self-signed certificate: %s, %s", certFile, keyFile)

	// Создаём директории если их нет
	if err := os.MkdirAll(filepath.Dir(certFile), 0755); err != nil {
		return err
	}

	// Получаем IP сервера из переменной окружения
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = "localhost"
	}

	// Команда для генерации сертификата с поддержкой IP
	var cmd string
	if isValidIP(serverIP) {
		// Для IP адреса используем SAN (Subject Alternative Name)
		cmd = fmt.Sprintf(`openssl req -x509 -newkey rsa:4096 -keyout %s -out %s -days 365 -nodes -subj "/C=RU/ST=Moscow/L=Moscow/O=Organization/CN=%s" -addext "subjectAltName=IP:%s"`, keyFile, certFile, serverIP, serverIP)
	} else {
		// Для домена или localhost
		cmd = fmt.Sprintf(`openssl req -x509 -newkey rsa:4096 -keyout %s -out %s -days 365 -nodes -subj "/C=RU/ST=Moscow/L=Moscow/O=Organization/CN=%s"`, keyFile, certFile, serverIP)
	}

	// Выполняем команду через shell
	if output, err := exec.Command("sh", "-c", cmd).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to generate certificate: %v, output: %s", err, string(output))
	}

	log.Printf("Self-signed certificate generated successfully for %s", serverIP)
	return nil
}

// isValidIP проверяет, является ли строка валидным IP адресом
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
