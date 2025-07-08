package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq" // Required for handling array parameters
)

// NewsletterUpdate matches the database table structure.
type NewsletterUpdate struct {
	ID          int64     `db:"id"`
	CommitHash  string    `db:"commit_hash"`
	Message     string    `db:"message"`
	Author      string    `db:"author"`
	CommittedAt time.Time `db:"committed_at"`
	IsProcessed bool      `db:"is_processed"`
	CreatedAt   time.Time `db:"created_at"`
}

// NewsletterUpdateStore defines the database operations for newsletter updates.
type NewsletterUpdateStore struct {
	db *sqlx.DB
}

// Create inserts a new update, ignoring conflicts on the commit hash.
func (s *NewsletterUpdateStore) Create(ctx context.Context, update *NewsletterUpdate) error {
	query := `
        INSERT INTO newsletter_updates (commit_hash, message, author, committed_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (commit_hash) DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, update.CommitHash, update.Message, update.Author, update.CommittedAt)
	return err
}

// GetUnprocessed retrieves all updates that have not yet been sent.
func (s *NewsletterUpdateStore) GetUnprocessed(ctx context.Context) ([]*NewsletterUpdate, error) {
	var updates []*NewsletterUpdate
	query := `SELECT id, message, author FROM newsletter_updates WHERE is_processed = FALSE ORDER BY committed_at ASC`
	err := s.db.SelectContext(ctx, &updates, query)
	return updates, err
}

// MarkAsProcessed updates the status of given update IDs.
func (s *NewsletterUpdateStore) MarkAsProcessed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	query := `UPDATE newsletter_updates SET is_processed = TRUE WHERE id = ANY($1)`
	_, err := s.db.ExecContext(ctx, query, pq.Array(ids))
	return err
}