package store

import (
	"context"
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
	AvatarUrl      string     `json:"avatar_url"`
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
			email, hashed_password, avatar_url, created_at, updated_at
		) VALUES (
			:username, :age, :first_name, :last_name, 
      :email, :hashed_password, :avatar_url, :created_at, :updated_at
		) RETURNING id`

	result, err := s.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.Id = id
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
	query := `SELECT * FROM users WHERE id = ?`
	err := s.db.GetContext(ctx, &user, query, id)
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
	query := `SELECT * FROM users WHERE username = ?`
	err := s.db.GetContext(ctx, &user, query, username)
	if err != nil {
		return nil, err
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
