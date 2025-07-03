// path: birdlens-be/internal/store/users.go
// This is the final, correct version of the file.
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
	Id                              int64      `json:"id" db:"id"`
	FirebaseUID                     *string    `json:"-" db:"firebase_uid"`
	SubscriptionId                  *int64     `json:"-" db:"subscription_id"`
	Username                        string     `json:"username" db:"username"`
	Age                             int        `json:"age" db:"age"`
	FirstName                       string     `json:"first_name" db:"first_name"`
	LastName                        string     `json:"last_name" db:"last_name"`
	Email                           string     `json:"email" db:"email"`
	HashedPassword                  *string    `json:"-" db:"hashed_password"`
	AuthProvider                    string     `json:"-" db:"auth_provider"`
	AvatarUrl                       *string    `json:"avatar_url" db:"avatar_url"`
	CreatedAt                       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                       *time.Time `json:"updated_at" db:"updated_at"`
	EmailVerified                   bool       `json:"email_verified" db:"email_verified"`
	EmailVerificationToken          *string    `json:"-" db:"email_verification_token"`
	EmailVerificationTokenExpiresAt *time.Time `json:"-" db:"email_verification_token_expires_at"`

	// Logic: The struct now perfectly matches the generic database schema after migrations.
	SubscriptionStatus    *string    `json:"-" db:"subscription_status"`
	SubscriptionPeriodEnd *time.Time `json:"-" db:"subscription_period_end"`
}

type UserStore struct {
	db *sqlx.DB
}

func (s *UserStore) Create(ctx context.Context, user *User) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
		INSERT INTO users (
			username, age, first_name, last_name, 
			email, hashed_password, avatar_url, firebase_uid, auth_provider, email_verified
		) VALUES (
      $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id, created_at`
	err := s.db.QueryRowContext(ctx, query, user.Username, user.Age, user.FirstName, user.LastName, user.Email, user.HashedPassword, user.AvatarUrl, user.FirebaseUID, user.AuthProvider, user.EmailVerified).Scan(&user.Id, &user.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

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
            avatar_url = :avatar_url,
            email_verified = :email_verified,
			updated_at = NOW()
		WHERE id = :id`

	_, err := s.db.NamedExecContext(ctx, query, user)
	return err
}

func (s *UserStore) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `DELETE FROM users WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *UserStore) GetById(ctx context.Context, id int64) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	// Logic: Corrected SELECT query to match the new generic schema.
	query := `
      SELECT id, firebase_uid, subscription_id, username, age, first_name, last_name, email, hashed_password, auth_provider, avatar_url, created_at, updated_at, email_verified, email_verification_token, email_verification_token_expires_at, subscription_status, subscription_period_end
      FROM users WHERE id = $1;
      `
	err := s.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	// Logic: Corrected SELECT query to match the new generic schema.
	query := `SELECT id, firebase_uid, subscription_id, username, age, first_name, last_name, email, hashed_password, auth_provider, avatar_url, created_at, updated_at, email_verified, email_verification_token, email_verification_token_expires_at, subscription_status, subscription_period_end
      FROM users WHERE username = $1`

	err := s.db.GetContext(ctx, &user, query, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	// Logic: Corrected SELECT query to match the new generic schema.
	query := `
        SELECT id, firebase_uid, subscription_id, username, age, first_name, last_name, email, hashed_password, auth_provider, avatar_url, created_at, updated_at, email_verified, email_verification_token, email_verification_token_expires_at, subscription_status, subscription_period_end
        FROM users
        WHERE email = $1
      `

	err := s.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) UsernameExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1);`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, username).Scan(&exists)
	if err != nil {
		log.Printf("Error checking if username '%s' exists: %v", username, err)
		return false, err
	}
	return exists, nil
}

func (s *UserStore) EmailExists(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, email).Scan(&exists)
	if err != nil {
		log.Printf("Error checking if email '%s' exists: %v", email, err)
		return false, err
	}
	return exists, nil
}

func (s *UserStore) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	// Logic: Corrected SELECT query to match the new generic schema.
	query := `SELECT id, firebase_uid, subscription_id, username, age, first_name, last_name, email, hashed_password, auth_provider, avatar_url, created_at, updated_at, email_verified, email_verification_token, email_verification_token_expires_at, subscription_status, subscription_period_end
        FROM users WHERE firebase_uid = $1`
	err := s.db.GetContext(ctx, &user, query, firebaseUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) AddEmailVerificationToken(ctx context.Context, userId int64, token string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
    UPDATE users
    SET email_verification_token = $1, email_verification_token_expires_at = $2
    WHERE id = $3;
  `

	_, err := s.db.ExecContext(ctx, query, token, expiresAt, userId)
	if err != nil {
		return err
	}

	return nil
}

func (s *UserStore) GetEmailVerificationToken(ctx context.Context, userId int64) (token string, expiresAt time.Time, err error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var dbToken sql.NullString
	var dbExpiresAt sql.NullTime

	query := `
    SELECT email_verification_token, email_verification_token_expires_at
    FROM users
    WHERE id = $1;
  `

	err = s.db.QueryRowContext(ctx, query, userId).Scan(&dbToken, &dbExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", time.Time{}, sql.ErrNoRows
		}
		return "", time.Time{}, err
	}
	if dbToken.Valid {
		token = dbToken.String
	}
	if dbExpiresAt.Valid {
		expiresAt = dbExpiresAt.Time
	}

	return token, expiresAt, nil
}

func (s *UserStore) VerifyUserEmail(ctx context.Context, userId int64) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
      UPDATE users
      SET email_verified = TRUE, email_verification_token = NULL, email_verification_token_expires_at = NULL
      WHERE id = $1;
      `
	_, err := s.db.ExecContext(ctx, query, userId)
	if err != nil {
		log.Printf("Error verifying user email for user ID %d: %v", userId, err)
		return err
	}

	return nil
}

func (s *UserStore) GetSubscriptionByName(ctx context.Context, name string) (*Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var sub Subscription
	query := `SELECT id, name, description, price, duration_days FROM subscriptions WHERE name = $1 LIMIT 1`
	err := s.db.GetContext(ctx, &sub, query, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("subscription with name '%s' not found", name)
		}
		return nil, err
	}
	return &sub, nil
}

func (s *UserStore) AddResetPasswordToken(ctx context.Context, email string, token string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
    UPDATE users
    SET forgot_password_token = $1, forgot_password_token_expires_at = $2
    WHERE email = $3;
  `

	_, err := s.db.ExecContext(ctx, query, token, expiresAt, email)
	if err != nil {
		return err
	}

	return nil
}

func (s *UserStore) GetUserByResetPasswordToken(ctx context.Context, token string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	// Logic: Corrected SELECT query to match the new generic schema.
	query := `
    SELECT id, firebase_uid, subscription_id, username, age, first_name, last_name, email, hashed_password, auth_provider, avatar_url, created_at, updated_at, email_verified, email_verification_token, email_verification_token_expires_at, subscription_status, subscription_period_end
    FROM users
    WHERE forgot_password_token = $1 AND forgot_password_token_expires_at > NOW()
  `

	err := s.db.GetContext(ctx, &user, query, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GrantSubscriptionForOrder(ctx context.Context, userID int64, subscriptionID int64) error {
	var subDurationDays int
	subQuery := `SELECT duration_days FROM subscriptions WHERE id = $1`
	err := s.db.GetContext(ctx, &subDurationDays, subQuery, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get subscription duration: %w", err)
	}

	periodEnd := time.Now().AddDate(0, 0, subDurationDays)

	query := `
		UPDATE users
		SET
			subscription_id = $1,
			subscription_status = 'active', 
			subscription_period_end = $2,
			updated_at = NOW()
		WHERE id = $3`

	_, err = s.db.ExecContext(ctx, query, subscriptionID, periodEnd, userID)
	if err != nil {
		log.Printf("Error updating user subscription for user ID %d: %v", userID, err)
		return err
	}
	log.Printf("Successfully granted subscription for user ID %d (Sub ID: %d)", userID, subscriptionID)
	return nil
}