package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Alias1177/Predictor/models"
)

// DB represents a database connection
type DB struct {
	*sql.DB
}

// ConnectionParams holds PostgreSQL connection parameters
type ConnectionParams struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// New creates a new database connection
func New(params ConnectionParams) (*DB, error) {
	// Create PostgreSQL connection string
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		params.Host, params.Port, params.User, params.Password, params.DBName, params.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Check connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// createTables creates the necessary tables if they don't exist
func createTables(db *sql.DB) error {
	// Create user subscriptions table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_subscriptions (
			user_id BIGINT PRIMARY KEY,
			chat_id BIGINT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			payment_id TEXT,
			stripe_subscription_id TEXT,
			currency_pair TEXT,
			timeframe TEXT,
			last_predicted TIMESTAMP
		)
	`)

	// Add the new column if it doesn't exist (for existing databases)
	_, _ = db.Exec(`
		ALTER TABLE user_subscriptions 
		ADD COLUMN IF NOT EXISTS stripe_subscription_id TEXT
	`)

	return err
}

// CreateSubscription creates a new subscription for a user
func (db *DB) CreateSubscription(userID, chatID int64, currencyPair, timeframe string) (*models.UserSubscription, error) {
	now := time.Now()
	sub := &models.UserSubscription{
		UserID:       userID,
		ChatID:       chatID,
		Status:       models.PaymentStatusPending,
		CreatedAt:    now,
		ExpiresAt:    now.AddDate(0, 1, 0), // 1 month from now
		CurrencyPair: currencyPair,
		Timeframe:    timeframe,
	}

	_, err := db.Exec(`
		INSERT INTO user_subscriptions (
			user_id, chat_id, status, created_at, expires_at, currency_pair, timeframe
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) 
		DO UPDATE SET 
			chat_id = EXCLUDED.chat_id,
			status = EXCLUDED.status,
			created_at = EXCLUDED.created_at,
			expires_at = EXCLUDED.expires_at, 
			currency_pair = EXCLUDED.currency_pair,
			timeframe = EXCLUDED.timeframe
	`,
		sub.UserID, sub.ChatID, sub.Status, sub.CreatedAt, sub.ExpiresAt, sub.CurrencyPair, sub.Timeframe)

	if err != nil {
		return nil, err
	}

	return sub, nil
}

// GetSubscription retrieves a user's subscription
func (db *DB) GetSubscription(userID int64) (*models.UserSubscription, error) {
	var sub models.UserSubscription
	var lastPredicted sql.NullTime
	var paymentID sql.NullString
	var stripeSubscriptionID sql.NullString

	err := db.QueryRow(`
		SELECT 
			user_id, chat_id, status, created_at, expires_at, 
			payment_id, stripe_subscription_id, currency_pair, timeframe, last_predicted
		FROM user_subscriptions
		WHERE user_id = $1
	`, userID).Scan(
		&sub.UserID, &sub.ChatID, &sub.Status, &sub.CreatedAt, &sub.ExpiresAt,
		&paymentID, &stripeSubscriptionID, &sub.CurrencyPair, &sub.Timeframe, &lastPredicted,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No subscription found
		}
		return nil, err
	}

	if paymentID.Valid {
		sub.PaymentID = paymentID.String
	}

	if stripeSubscriptionID.Valid {
		sub.StripeSubscriptionID = stripeSubscriptionID.String
	}

	if lastPredicted.Valid {
		sub.LastPredicted = lastPredicted.Time
	}

	return &sub, nil
}

// UpdateSubscriptionStatus updates a user's subscription status
func (db *DB) UpdateSubscriptionStatus(userID int64, status string, paymentID string) error {
	_, err := db.Exec(`
		UPDATE user_subscriptions
		SET status = $1, payment_id = $2
		WHERE user_id = $3
	`, status, paymentID, userID)

	return err
}

// CheckAndUpdateExpirations checks for expired subscriptions and updates their status
func (db *DB) CheckAndUpdateExpirations() error {
	_, err := db.Exec(`
		UPDATE user_subscriptions
		SET status = $1
		WHERE status = $2 AND expires_at <= NOW()
	`, models.PaymentStatusClosed, models.PaymentStatusAccepted)

	return err
}

// CloseSubscription closes a user's subscription
func (db *DB) CloseSubscription(userID int64) error {
	_, err := db.Exec(`
		UPDATE user_subscriptions
		SET status = $1
		WHERE user_id = $2
	`, models.PaymentStatusClosed, userID)

	return err
}

// UpdateLastPredicted updates the last time a user made a prediction
func (db *DB) UpdateLastPredicted(userID int64) error {
	_, err := db.Exec(`
		UPDATE user_subscriptions
		SET last_predicted = NOW()
		WHERE user_id = $1
	`, userID)

	return err
}

// UpdateStripeSubscriptionID updates the Stripe subscription ID for a user
func (db *DB) UpdateStripeSubscriptionID(userID int64, stripeSubscriptionID string) error {
	_, err := db.Exec(`
		UPDATE user_subscriptions
		SET stripe_subscription_id = $1
		WHERE user_id = $2
	`, stripeSubscriptionID, userID)

	return err
}

// GetStripeSubscriptionID gets the Stripe subscription ID for a user
func (db *DB) GetStripeSubscriptionID(userID int64) (string, error) {
	var stripeSubscriptionID sql.NullString

	err := db.QueryRow(`
		SELECT stripe_subscription_id
		FROM user_subscriptions
		WHERE user_id = $1
	`, userID).Scan(&stripeSubscriptionID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	if stripeSubscriptionID.Valid {
		return stripeSubscriptionID.String, nil
	}

	return "", nil
}
