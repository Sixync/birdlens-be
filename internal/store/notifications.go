package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// Notification represents a notification for a user.
type Notification struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Type      string    `json:"type" db:"type"`
	Message   string    `json:"message" db:"message"`
	IsRead    bool      `json:"is_read" db:"is_read"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NotificationStore defines database operations for notifications.
type NotificationStore struct {
	db *sqlx.DB
}

// Create inserts a new notification into the database.
func (s *NotificationStore) Create(ctx context.Context, notification *Notification) error {
	query := `INSERT INTO notifications (user_id, type, message) VALUES ($1, $2, $3) RETURNING id, created_at`
	err := s.db.QueryRowContext(ctx, query, notification.UserID, notification.Type, notification.Message).Scan(notification.ID, notification.CreatedAt)
	return err
}

// GetByUserID fetches notifications for a specific user with pagination.
func (s *NotificationStore) GetByUserID(ctx context.Context, userID int64, limit, offset int) (*PaginatedList[*Notification], error) {
	var notifications []*Notification
	query := `SELECT id, user_id, type, message, is_read, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	err := s.db.SelectContext(ctx, &notifications, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	err = s.db.GetContext(ctx, &totalCount, countQuery, userID)
	if err != nil {
		return nil, err
	}

	return NewPaginatedList(notifications, totalCount, limit, offset)
}