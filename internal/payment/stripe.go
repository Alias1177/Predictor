package payment

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/Alias1177/Predictor/models"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"
)

// StripeService handles Stripe payment operations
type StripeService struct {
	SubscriptionPriceID string
	WebhookSecret       string
}

// NewStripeService creates a new Stripe payment service
func NewStripeService() *StripeService {
	// Initialize Stripe with the API key
	stripe.Key = os.Getenv("STRIPE_API_KEY")

	service := &StripeService{
		SubscriptionPriceID: os.Getenv("STRIPE_SUBSCRIPTION_PRICE_ID"),
		WebhookSecret:       os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}

	// Validate configuration
	if err := service.ValidateConfig(); err != nil {
		fmt.Printf("WARNING: Stripe configuration validation failed: %v\n", err)
	}

	return service
}

// ValidateConfig validates the Stripe service configuration
func (s *StripeService) ValidateConfig() error {
	if stripe.Key == "" {
		return fmt.Errorf("STRIPE_API_KEY not set")
	}

	if s.SubscriptionPriceID == "" {
		return fmt.Errorf("STRIPE_SUBSCRIPTION_PRICE_ID not set")
	}

	if s.WebhookSecret == "" {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET not set")
	}

	botUsername := os.Getenv("TELEGRAM_BOT_USERNAME")
	if botUsername == "" {
		return fmt.Errorf("TELEGRAM_BOT_USERNAME not set")
	}

	fmt.Printf("Stripe configuration validated successfully:\n")
	fmt.Printf("  - API Key: %s...%s\n", stripe.Key[:8], stripe.Key[len(stripe.Key)-4:])
	fmt.Printf("  - Price ID: %s\n", s.SubscriptionPriceID)
	fmt.Printf("  - Bot Username: %s\n", botUsername)
	fmt.Printf("  - Webhook Secret: %s...%s\n", s.WebhookSecret[:8], s.WebhookSecret[len(s.WebhookSecret)-4:])

	return nil
}

// CreateCheckoutSession creates a new Stripe checkout session for a subscription
func (s *StripeService) CreateCheckoutSession(userID int64, currencyPair, timeframe string) (string, string, error) {
	// Validate required fields
	if s.SubscriptionPriceID == "" {
		return "", "", fmt.Errorf("STRIPE_SUBSCRIPTION_PRICE_ID not set")
	}

	botUsername := os.Getenv("TELEGRAM_BOT_USERNAME")
	if botUsername == "" {
		return "", "", fmt.Errorf("TELEGRAM_BOT_USERNAME not set")
	}

	fmt.Printf("Creating checkout session for user %d, price ID: %s\n", userID, s.SubscriptionPriceID)

	// Set success and cancel URLs
	successURL := fmt.Sprintf("https://t.me/%s?start=payment_success", botUsername)
	cancelURL := fmt.Sprintf("https://t.me/%s?start=payment_cancel", botUsername)

	// Create metadata for the session
	metadata := map[string]string{
		"user_id":       fmt.Sprintf("%d", userID),
		"currency_pair": currencyPair,
		"timeframe":     timeframe,
	}

	fmt.Printf("Checkout session metadata: %+v\n", metadata)

	// Create checkout session parameters
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(fmt.Sprintf("%d", userID)), // КРИТИЧНО для поиска подписок!
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(s.SubscriptionPriceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: metadata,
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: metadata,
		},
	}

	// Create the checkout session
	sess, err := session.New(params)
	if err != nil {
		fmt.Printf("Failed to create checkout session: %v\n", err)
		return "", "", fmt.Errorf("failed to create checkout session: %v", err)
	}

	fmt.Printf("Successfully created checkout session: ID=%s, URL=%s\n", sess.ID, sess.URL)
	return sess.ID, sess.URL, nil
}

// VerifyWebhookSignature verifies the signature of a Stripe webhook event
func (s *StripeService) VerifyWebhookSignature(payload []byte, signature string) (*stripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, signature, s.WebhookSecret)
	return &event, err
}

// ProcessSubscriptionPayment processes a Stripe subscription payment webhook event
func (s *StripeService) ProcessSubscriptionPayment(event *stripe.Event) (int64, string, string, error) {
	// Process different event types
	switch event.Type {
	case "checkout.session.completed":
		// Payment was successful (works for both subscription and one-time payments)
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return 0, "", "", fmt.Errorf("failed to parse checkout session: %v", err)
		}

		// Детальное логирование
		fmt.Printf("Checkout session details:\n")
		fmt.Printf("  ID: %s\n", sess.ID)
		fmt.Printf("  ClientReferenceID: %s\n", sess.ClientReferenceID)
		fmt.Printf("  Metadata: %+v\n", sess.Metadata)
		fmt.Printf("  Subscription ID: %v\n", sess.Subscription)
		if sess.Subscription != nil {
			fmt.Printf("  Subscription.ID: %s\n", sess.Subscription.ID)
		}

		// Сначала пытаемся получить user_id из client_reference_id
		var userID int64
		var err error

		if sess.ClientReferenceID != "" {
			userID, err = strconv.ParseInt(sess.ClientReferenceID, 10, 64)
			if err != nil {
				fmt.Printf("Invalid ClientReferenceID: %s, trying metadata\n", sess.ClientReferenceID)
			} else {
				fmt.Printf("Found userID from ClientReferenceID: %d\n", userID)
			}
		}

		// Fallback на metadata если ClientReferenceID не работает
		if userID == 0 {
			userIDStr, ok := sess.Metadata["user_id"]
			if !ok {
				return 0, "", "", fmt.Errorf("user_id not found in session ClientReferenceID or metadata")
			}

			userID, err = strconv.ParseInt(userIDStr, 10, 64)
			if err != nil {
				return 0, "", "", fmt.Errorf("invalid user_id: %v", err)
			}
			fmt.Printf("Found userID from metadata: %d\n", userID)
		}

		// Extract subscription ID if available
		subscriptionID := ""
		if sess.Subscription != nil {
			subscriptionID = sess.Subscription.ID
		}

		// Handle promo code usage if present
		if promoCode, exists := sess.Metadata["promo_code"]; exists && promoCode != "" {
			if discountStr, discountExists := sess.Metadata["discount"]; discountExists {
				if discount, parseErr := strconv.ParseFloat(discountStr, 64); parseErr == nil {
					fmt.Printf("Promo code %s used by user %d with discount $%.2f\n", promoCode, userID, discount)
				}
			}
		}

		return userID, models.PaymentStatusAccepted, subscriptionID, nil

	case "invoice.payment_succeeded":
		// Повторяющийся платеж по подписке
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return 0, "", "", fmt.Errorf("failed to parse invoice: %v", err)
		}

		fmt.Printf("Invoice payment succeeded: ID=%s\n", invoice.ID)

		// Получаем подписку
		if invoice.Subscription == nil {
			return 0, "", "", fmt.Errorf("no subscription in invoice")
		}

		// Получаем подписку из Stripe
		sub, err := subscription.Get(invoice.Subscription.ID, nil)
		if err != nil {
			return 0, "", "", fmt.Errorf("failed to get subscription: %v", err)
		}

		fmt.Printf("Subscription details: ID=%s, Status=%s, Metadata=%+v\n",
			sub.ID, sub.Status, sub.Metadata)

		// Извлекаем user_id из метаданных подписки
		userIDStr, ok := sub.Metadata["user_id"]
		if !ok {
			return 0, "", "", fmt.Errorf("user_id not found in subscription metadata")
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return 0, "", "", fmt.Errorf("invalid user_id: %v", err)
		}

		fmt.Printf("Renewing subscription for user %d, subscription ID: %s\n", userID, sub.ID)

		// Продлеваем подписку на месяц
		return userID, models.PaymentStatusAccepted, sub.ID, nil

	case "customer.subscription.created":
		// Подписка создана
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return 0, "", "", fmt.Errorf("failed to parse subscription: %v", err)
		}

		fmt.Printf("Subscription created: ID=%s, Status=%s, Metadata=%+v\n",
			sub.ID, sub.Status, sub.Metadata)

		// Обновляем stripe_subscription_id в базе, но не меняем статус
		// Статус изменится когда придет checkout.session.completed
		userIDStr, ok := sub.Metadata["user_id"]
		if ok {
			if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
				fmt.Printf("Subscription created for user %d: %s\n", userID, sub.ID)
				// Возвращаем 0 для userID чтобы не обновлять статус, только сохранить ID подписки
				return 0, "", sub.ID, nil
			}
		}

		return 0, "", "", nil // Не обрабатываем если нет user_id

	case "payment_intent.succeeded":
		// Payment intent succeeded (for one-time payments)
		var intent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &intent); err != nil {
			return 0, "", "", fmt.Errorf("failed to parse payment intent: %v", err)
		}

		// Extract user ID from metadata
		userIDStr, ok := intent.Metadata["user_id"]
		if !ok {
			// Try to find it in the description or elsewhere
			fmt.Printf("User ID not found in metadata, checking description: %s\n", intent.Description)

			// If we can't find it, log but don't error out yet
			if !ok {
				fmt.Printf("Warning: user_id not found in payment intent metadata\n")
				// Try to proceed with other data
				return 0, "", "", fmt.Errorf("user_id not found in payment intent metadata")
			}
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return 0, "", "", fmt.Errorf("invalid user_id: %v", err)
		}

		return userID, models.PaymentStatusAccepted, "", nil

	case "customer.subscription.deleted":
		// Subscription was cancelled or expired
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return 0, "", "", fmt.Errorf("failed to parse subscription: %v", err)
		}

		// Extract user ID from metadata (if available)
		userIDStr, ok := sub.Metadata["user_id"]
		if !ok {
			return 0, "", "", fmt.Errorf("user_id not found in subscription metadata")
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return 0, "", "", fmt.Errorf("invalid user_id: %v", err)
		}

		return userID, models.PaymentStatusClosed, sub.ID, nil

	case "charge.succeeded":
		// Charge succeeded (another payment event)
		var charge stripe.Charge
		if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
			return 0, "", "", fmt.Errorf("failed to parse charge: %v", err)
		}

		// Try to extract user ID from metadata
		userIDStr, ok := charge.Metadata["user_id"]
		if !ok {
			fmt.Printf("Warning: user_id not found in charge metadata\n")
			// We might try other lookups or return an error
			return 0, "", "", fmt.Errorf("user_id not found in charge metadata")
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return 0, "", "", fmt.Errorf("invalid user_id: %v", err)
		}

		return userID, models.PaymentStatusAccepted, "", nil

	default:
		fmt.Printf("Unhandled event type: %s\n", event.Type)
		return 0, "", "", fmt.Errorf("unhandled event type: %s", event.Type)
	}
}

// CancelSubscription cancels a user's Stripe subscription
func (s *StripeService) CancelSubscription(subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is empty")
	}

	fmt.Printf("Attempting to cancel Stripe subscription: %s\n", subscriptionID)

	// Cancel the subscription immediately
	params := &stripe.SubscriptionCancelParams{}

	canceledSub, err := subscription.Cancel(subscriptionID, params)
	if err != nil {
		fmt.Printf("Failed to cancel subscription %s: %v\n", subscriptionID, err)
		return fmt.Errorf("failed to cancel subscription: %v", err)
	}

	fmt.Printf("Successfully cancelled subscription %s. Status: %s\n", subscriptionID, canceledSub.Status)
	return nil
}

// GetSubscriptionByCustomer retrieves subscription by customer ID
func (s *StripeService) GetSubscriptionByCustomer(customerID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionListParams{
		Customer: stripe.String(customerID),
		Status:   stripe.String("active"),
	}

	iter := subscription.List(params)
	for iter.Next() {
		subscription := iter.Subscription()
		return subscription, nil
	}

	if iter.Err() != nil {
		return nil, iter.Err()
	}

	return nil, fmt.Errorf("no active subscription found")
}

// FindSubscriptionByUserID attempts to find an active subscription for a user
// by searching through checkout sessions and subscription metadata
func (s *StripeService) FindSubscriptionByUserID(userID int64) (*stripe.Subscription, error) {
	userIDStr := fmt.Sprintf("%d", userID)

	fmt.Printf("Searching for subscription for user %d\n", userID)

	// Сначала ищем все checkout sessions с нашим client_reference_id
	sessionParams := &stripe.CheckoutSessionListParams{}
	sessionParams.Filters.AddFilter("limit", "", "100")

	sessionIter := session.List(sessionParams)
	for sessionIter.Next() {
		sess := sessionIter.CheckoutSession()

		// Проверяем client_reference_id
		if sess.ClientReferenceID == userIDStr && sess.Subscription != nil {
			// Нашли сессию с нашим userID, получаем подписку
			sub, err := subscription.Get(sess.Subscription.ID, nil)
			if err == nil && sub.Status != "canceled" {
				fmt.Printf("Found subscription via ClientReferenceID: %s (status: %s)\n", sub.ID, sub.Status)
				return sub, nil
			}
		}
	}

	if sessionIter.Err() != nil {
		fmt.Printf("Error searching checkout sessions: %v\n", sessionIter.Err())
	}

	// Fallback на поиск по метаданным подписки
	fmt.Printf("Searching subscriptions by metadata for user %d\n", userID)

	// First try active subscriptions
	params := &stripe.SubscriptionListParams{
		Status: stripe.String("active"),
	}
	params.Filters.AddFilter("limit", "", "100")

	iter := subscription.List(params)
	for iter.Next() {
		sub := iter.Subscription()

		// Check if metadata contains our user_id
		if sub.Metadata != nil {
			if metaUserID, exists := sub.Metadata["user_id"]; exists && metaUserID == userIDStr {
				fmt.Printf("Found active subscription via metadata for user %d: %s\n", userID, sub.ID)
				return sub, nil
			}
		}
	}

	if iter.Err() != nil {
		return nil, iter.Err()
	}

	// If no active subscription found, try all subscriptions (including past_due, cancelled, etc.)
	fmt.Printf("No active subscription found, searching all subscriptions for user %d\n", userID)

	allParams := &stripe.SubscriptionListParams{}
	allParams.Filters.AddFilter("limit", "", "100")

	allIter := subscription.List(allParams)
	for allIter.Next() {
		sub := allIter.Subscription()

		// Check if metadata contains our user_id
		if sub.Metadata != nil {
			if metaUserID, exists := sub.Metadata["user_id"]; exists && metaUserID == userIDStr {
				fmt.Printf("Found subscription (status: %s) for user %d: %s\n", sub.Status, userID, sub.ID)
				// Only return if it's not already cancelled
				if sub.Status != "canceled" {
					return sub, nil
				}
				fmt.Printf("Subscription %s is already cancelled, skipping\n", sub.ID)
			}
		}
	}

	if allIter.Err() != nil {
		return nil, allIter.Err()
	}

	return nil, fmt.Errorf("no subscription found for user %d", userID)
}

// FindSubscriptionAdvanced attempts to find subscription using multiple methods
func (s *StripeService) FindSubscriptionAdvanced(userID int64, createdAfter int64) (*stripe.Subscription, error) {
	fmt.Printf("Starting advanced search for user %d, created after %d\n", userID, createdAfter)

	// First try metadata search
	if sub, err := s.FindSubscriptionByUserID(userID); err == nil {
		fmt.Printf("Found subscription via metadata: %s\n", sub.ID)
		return sub, nil
	}

	// Try searching by creation time (recent subscriptions)
	// Don't filter by status - search all subscriptions
	params := &stripe.SubscriptionListParams{}
	params.Filters.AddFilter("limit", "", "50")
	if createdAfter > 0 {
		params.Filters.AddFilter("created[gte]", "", fmt.Sprintf("%d", createdAfter))
	}

	userIDStr := fmt.Sprintf("%d", userID)

	fmt.Printf("Searching through active subscriptions created after %d\n", createdAfter)

	iter := subscription.List(params)
	var candidates []*stripe.Subscription

	for iter.Next() {
		sub := iter.Subscription()

		fmt.Printf("Checking subscription %s, created: %d\n", sub.ID, sub.Created)

		// Check metadata first
		if sub.Metadata != nil {
			if metaUserID, exists := sub.Metadata["user_id"]; exists && metaUserID == userIDStr {
				fmt.Printf("Found subscription via metadata in time range: %s (status: %s)\n", sub.ID, sub.Status)
				// Only return if not already cancelled
				if sub.Status != "canceled" {
					return sub, nil
				}
				fmt.Printf("Subscription %s is already cancelled, continuing search\n", sub.ID)
			}

			// Log all metadata for debugging
			fmt.Printf("Subscription %s metadata: %+v\n", sub.ID, sub.Metadata)
		}

		// Collect recent subscriptions as candidates (excluding cancelled ones)
		if sub.Created >= createdAfter && sub.Status != "canceled" {
			candidates = append(candidates, sub)
		}
	}

	if iter.Err() != nil {
		return nil, iter.Err()
	}

	// If we found recent subscriptions but no metadata match
	if len(candidates) > 0 {
		fmt.Printf("Found %d candidate subscriptions, but no metadata match\n", len(candidates))

		// Return the most recent one if there's only one candidate
		if len(candidates) == 1 {
			fmt.Printf("Only one candidate found, assuming it's the right one: %s\n", candidates[0].ID)
			return candidates[0], nil
		}
	}

	return nil, fmt.Errorf("no matching subscription found for user %d", userID)
}

// ListAllSubscriptionsForUser lists all subscriptions for a user (for debugging)
func (s *StripeService) ListAllSubscriptionsForUser(userID int64) ([]*stripe.Subscription, error) {
	userIDStr := fmt.Sprintf("%d", userID)
	var result []*stripe.Subscription

	fmt.Printf("Listing all subscriptions for user %d\n", userID)

	// Search through all subscriptions
	params := &stripe.SubscriptionListParams{}
	params.Filters.AddFilter("limit", "", "100")

	iter := subscription.List(params)
	for iter.Next() {
		sub := iter.Subscription()

		// Check if this subscription belongs to our user
		if sub.Metadata != nil {
			if metaUserID, exists := sub.Metadata["user_id"]; exists && metaUserID == userIDStr {
				fmt.Printf("Found subscription for user %d: ID=%s, Status=%s, Created=%d\n",
					userID, sub.ID, sub.Status, sub.Created)
				result = append(result, sub)
			}
		}
	}

	if iter.Err() != nil {
		return nil, iter.Err()
	}

	fmt.Printf("Total subscriptions found for user %d: %d\n", userID, len(result))
	return result, nil
}
