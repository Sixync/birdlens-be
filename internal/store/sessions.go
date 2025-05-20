package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type Session struct {
	ID           int64     `json:"id"`
	UserEmail    string    `json:"user_email"`
	RefreshToken string    `json:"refresh_token"`
	IsRevoked    bool      `json:"is_revoked"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SessionStore struct {
	db *sqlx.DB
}

func NewSessionStore(db *sqlx.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(ctx context.Context, session *Session) error {
	// Implement the logic to create a new session in the database
	query := `INSERT INTO sessions (id, user_email, refresh_token, is_revoked, expires_at) 
        VALUES ($1, $2, $3, $4, $5) RETURNING created_at;`
	err := s.db.QueryRowContext(ctx, query, session.ID, session.UserEmail, session.RefreshToken,
		session.IsRevoked, session.ExpiresAt).Scan(&session.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *SessionStore) GetById(ctx context.Context, sessionId int64) (*Session, error) {
	// Implement the logic to get a session by ID from the database
	query := `SELECT id, user_email, refresh_token, is_revoked, expires_at, created_at, updated_at 
        FROM sessions WHERE id = $1`
	session := &Session{}
	err := s.db.QueryRowContext(ctx, query, sessionId).Scan(&session.ID, &session.UserEmail,
		&session.RefreshToken, &session.IsRevoked, &session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("session not found: %w", err)
		default:
			return nil, err
		}
	}
	return session, nil
}

func (s *SessionStore) GetByUserEmail(ctx context.Context, userEmail string) (*Session, error) {
	// Implement the logic to get a session by user email from the database
	query := `SELECT id, user_email, refresh_token, is_revoked, expires_at, created_at, updated_at
        FROM sessions WHERE user_email = $1`
	session := &Session{}
	err := s.db.QueryRowContext(ctx, query, userEmail).Scan(&session.ID, &session.UserEmail,
		&session.RefreshToken, &session.IsRevoked, &session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SessionStore) RevokeSession(ctx context.Context, sessionId int64) error {
	// Implement the logic to revoke a session in the database
	query := `UPDATE sessions SET is_revoked = true WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, sessionId)
	return err
}

func (s *SessionStore) DeleteSession(ctx context.Context, sessionId int64) error {
	// Implement the logic to delete a session from the database
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, sessionId)
	return err
}
