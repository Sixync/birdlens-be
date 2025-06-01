package store

import (
	"context"
	"log"

	"github.com/jmoiron/sqlx"
)

type Subscription struct {
	ID           int64   `json:"id" db:"id"`
	Name         string  `json:"name" db:"name"`
	Description  string  `json:"description" db:"description"`
	Price        float64 `json:"price" db:"price"`
	DurationDays int     `json:"duration_days" db:"duration_days"`
	CreatedAt    string  `json:"created_at" db:"created_at"`
	UpdatedAt    *string `json:"updated_at" db:"updated_at"`
}

type SubscriptionStore struct {
	db *sqlx.DB
}

func (s *SubscriptionStore) GetUserSubscriptionByEmail(
	ctx context.Context,
	email string,
) (*Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	var subscription Subscription
	log.Printf("Fetching subscription for user with email: %s", email)

	query := `SELECT s.id, s.name, s.description, s.price, s.duration_days,
            s.created_at, s.updated_at 
            FROM subscriptions s 
            JOIN users u ON u.subscription_id = s.id 
            WHERE u.email = $1`

	err := s.db.GetContext(ctx, &subscription, query, email)
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

func (s *SubscriptionStore) GetAll(ctx context.Context) ([]*Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var subscriptions []*Subscription
	query := `SELECT id, name, description, price, duration_days, created_at, updated_at FROM subscriptions`
	err := s.db.SelectContext(ctx, &subscriptions, query)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *SubscriptionStore) Create(ctx context.Context, subscription *Subscription) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `INSERT INTO subscriptions (name, description, price, duration_days) 
              VALUES ($1, $2, $3, $4) RETURNING id`
	err := s.db.QueryRowContext(ctx, query,
		subscription.Name,
		subscription.Description,
		subscription.Price,
		subscription.DurationDays).Scan(&subscription.ID)
	if err != nil {
		return err
	}

	return nil
}
