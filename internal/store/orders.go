// path: birdlens-be/internal/store/orders.go
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type Order struct {
	ID              int64      `db:"id"`
	UserID          int64      `db:"user_id"`
	SubscriptionID  int64      `db:"subscription_id"`
	PaymentGateway  string     `db:"payment_gateway"`
	GatewayOrderID  string     `db:"gateway_order_id"`
	Amount          int64      `db:"amount"`
	Currency        string     `db:"currency"`
	Status          string     `db:"status"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       *time.Time `db:"updated_at"`
}

type OrderStore struct {
	db *sqlx.DB
}

const (
	OrderStatusPending   = "PENDING"
	OrderStatusPaid      = "PAID"
	OrderStatusFailed    = "FAILED"
	OrderStatusCancelled = "CANCELLED"
)

func (s *OrderStore) Create(ctx context.Context, order *Order) error {
	query := `INSERT INTO orders (user_id, subscription_id, payment_gateway, gateway_order_id, amount, currency, status)
              VALUES ($1, $2, $3, $4, $5, $6, $7)
              RETURNING id, created_at`
	err := s.db.QueryRowContext(ctx, query, order.UserID, order.SubscriptionID, order.PaymentGateway, order.GatewayOrderID, order.Amount, order.Currency, order.Status).Scan(&order.ID, &order.CreatedAt)
	return err
}

func (s *OrderStore) GetByGatewayOrderID(ctx context.Context, gatewayOrderID string) (*Order, error) {
	var order Order
	query := `SELECT * FROM orders WHERE gateway_order_id = $1`
	err := s.db.GetContext(ctx, &order, query, gatewayOrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &order, nil
}

func (s *OrderStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, status, id)
	return err
}