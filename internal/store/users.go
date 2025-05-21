package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type User struct {
	Id             int64      `json:"id"`
	Username       string     `json:"username"`
	Age            int        `json:"age"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	Email          string     `json:"email"`
	HashedPassword string     `json:"-"`
	AvatarUrl      *string    `json:"avatar_url"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
}

type UserStore struct {
	db *sqlx.DB
}

// Create inserts a new user into the database
func (s *UserStore) Create(ctx context.Context, user *User) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
		INSERT INTO users (
			username, age, first_name, last_name, 
			email, hashed_password, avatar_url 
		) VALUES (
      $1, $2, $3, $4, $5, $6, $7 
		) RETURNING id, created_at`

	err := s.db.QueryRowContext(ctx, query, user.Username, user.Age, user.FirstName, user.LastName, user.Email, user.HashedPassword, user.AvatarUrl).Scan(&user.Id, &user.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

// Update modifies an existing user in the database
func (s *UserStore) Update(ctx context.Context, user *User) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
		UPDATE users
		SET
			username = :username,
			age = :age,
			first_name = :first_name,
			last_name = :last_name,
			email = :email,
			hashed_password = :hashed_password,
			updated_at = :updated_at
		WHERE id = :id`

	_, err := s.db.NamedExecContext(ctx, query, user)
	return err
}

// Delete removes a user from the database by ID
func (s *UserStore) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `DELETE FROM users WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// Get retrieves a user by ID
func (s *UserStore) GetById(ctx context.Context, id int64) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	query := `
  SELECT username, age, first_name, last_name, email, hashed_password, avatar_url, created_at, updated_at
  FROM users WHERE id = $1;
  `
	err := s.db.QueryRowContext(ctx, query, id).Scan(&user.Username, &user.Age, &user.FirstName, &user.LastName, &user.Email, &user.HashedPassword, &user.AvatarUrl, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername retrieves a user by username
func (s *UserStore) GetByUsername(ctx context.Context, username string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	query := `SELECT id, username, age, first_name, last_name, email, hashed_password, created_at, updated_at, avatar_url
  FROM users WHERE username = $1`

	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.Id,
		&user.Username,
		&user.Age,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.HashedPassword,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.AvatarUrl,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("user not found: %w", err)
		default:
			return nil, err
		}
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	query := `SELECT * FROM users WHERE email = ?`
	err := s.db.GetContext(ctx, &user, query, email)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// create check exist user
func (s *UserStore) UsernameExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1);`
	var exists bool

	err := s.db.QueryRowContext(ctx, query, username).Scan(&exists)
	if err != nil {
		// sql.ErrNoRows should generally not happen here because EXISTS always returns one row
		// with a boolean value. If it does, it's an unexpected DB or driver behavior.
		log.Printf("Error checking if email '%s' exists: %v", username, err)
		return false, err // Propagate the actual db error
	}
	return exists, nil
}

// create check exist user email
func (s *UserStore) EmailExists(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);`
	var exists bool

	err := s.db.QueryRowContext(ctx, query, email).Scan(&exists)
	if err != nil {
		// sql.ErrNoRows should generally not happen here because EXISTS always returns one row
		// with a boolean value. If it does, it's an unexpected DB or driver behavior.
		log.Printf("Error checking if email '%s' exists: %v", email, err)
		return false, err // Propagate the actual db error
	}
	return exists, nil
}
