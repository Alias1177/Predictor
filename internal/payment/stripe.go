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

	return &StripeService{
		SubscriptionPriceID: os.Getenv("STRIPE_SUBSCRIPTION_PRICE_ID"),
		WebhookSecret:       os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
}

// CreateCheckoutSession creates a new Stripe checkout session for a subscription
func (s *StripeService) CreateCheckoutSession(userID int64, currencyPair, timeframe string) (string, string, error) {
	// Set success and cancel URLs
	botUsername := os.Getenv("TELEGRAM_BOT_USERNAME")
	successURL := fmt.Sprintf("https://t.me/%s?start=payment_success", botUsername)
	cancelURL := fmt.Sprintf("https://t.me/%s?start=payment_cancel", botUsername)

	// Create metadata for the session
	metadata := map[string]string{
		"user_id":       fmt.Sprintf("%d", userID),
		"currency_pair": currencyPair,
		"timeframe":     timeframe,
	}

	// Create checkout session parameters
	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(s.SubscriptionPriceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: metadata,
	}

	// Create the checkout session
	sess, err := session.New(params)
	if err != nil {
		return "", "", err
	}

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

		// Log the session details for debugging
		fmt.Printf("Checkout session completed: ID=%s, Mode=%s\n", sess.ID, sess.Mode)

		// Extract user ID from metadata
		userIDStr, ok := sess.Metadata["user_id"]
		if !ok {
			return 0, "", "", fmt.Errorf("user_id not found in session metadata")
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return 0, "", "", fmt.Errorf("invalid user_id: %v", err)
		}

		// Extract subscription ID if available
		subscriptionID := ""
		if sess.Subscription != nil {
			subscriptionID = sess.Subscription.ID
		}

		return userID, models.PaymentStatusAccepted, subscriptionID, nil

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
	// Cancel the subscription immediately
	params := &stripe.SubscriptionCancelParams{}

	_, err := subscription.Cancel(subscriptionID, params)
	return err
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
