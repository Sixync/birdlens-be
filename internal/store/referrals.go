package store

import (
    "context"
    "database/sql"
    "errors"
    "time"

    "github.com/jmoiron/sqlx"
)

// Referral tracks the relationship between a referrer and a new user (referee).
type Referral struct {
    ID                int64      `db:"id"`
    ReferrerID        int64      `db:"referrer_id"`
    RefereeID         int64      `db:"referee_id"`
    Status            string     `db:"status"`
    ReferralCodeUsed  string     `db:"referral_code_used"`
    CreatedAt         time.Time  `db:"created_at"`
    CompletedAt       *time.Time `db:"completed_at"`
}

const (
    ReferralStatusPending   = "pending"
    ReferralStatusCompleted = "completed"
)

// ReferralStore defines the database operations for referrals.
type ReferralStore struct {
    db *sqlx.DB
}

// Create inserts a new pending referral record.
func (s *ReferralStore) Create(ctx context.Context, referral *Referral) error {
    query := `INSERT INTO referrals (referrer_id, referee_id, status, referral_code_used)
              VALUES ($1, $2, $3, $4)
              RETURNING id, created_at`
    err := s.db.QueryRowContext(ctx, query, referral.ReferrerID, referral.RefereeID, referral.Status, referral.ReferralCodeUsed).Scan(&referral.ID, &referral.CreatedAt)
    return err
}

// GetPendingByRefereeID finds a pending referral for a new user.
func (s *ReferralStore) GetPendingByRefereeID(ctx context.Context, refereeID int64) (*Referral, error) {
    var referral Referral
    query := `SELECT * FROM referrals WHERE referee_id = $1 AND status = 'pending'`
    err := s.db.GetContext(ctx, &referral, query, refereeID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, sql.ErrNoRows
        }
        return nil, err
    }
    return &referral, nil
}

// Complete updates a referral's status to 'completed'.
func (s *ReferralStore) Complete(ctx context.Context, id int64) error {
    query := `UPDATE referrals SET status = 'completed', completed_at = NOW() WHERE id = $1`
    _, err := s.db.ExecContext(ctx, query, id)
    return err
}