package store

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
)

type Follower struct {
	UserId     int64 `json:"user_id" db:"user_id"`
	User       *User `json:"user,omitempty"`
	FollowerId int64 `json:"follower_id" db:"follower_id"`
	Follower   *User `json:"follower,omitempty"`
}

type FollowerStore struct {
	db *sqlx.DB
}

// Create inserts a new follower relationship into the database
func (s *FollowerStore) Create(ctx context.Context, follower *Follower) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if follower == nil {
		return errors.New("follower cannot be nil")
	}
	if follower.UserId == 0 {
		return errors.New("user_id is required")
	}
	if follower.FollowerId == 0 {
		return errors.New("follower_id is required")
	}
	if follower.UserId == follower.FollowerId {
		return errors.New("user_id and follower_id cannot be the same")
	}

	query := `
		INSERT INTO followers (user_id, follower_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, follower_id) DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, follower.UserId, follower.FollowerId)
	if err != nil {
		return err
	}

	// Verify the insertion
	var exists bool
	verifyQuery := `SELECT EXISTS (SELECT 1 FROM followers WHERE user_id = $1 AND follower_id = $2)`
	err = s.db.GetContext(ctx, &exists, verifyQuery, follower.UserId, follower.FollowerId)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("failed to create follower relationship: may already exist or foreign key constraint violated")
	}

	return nil
}

// Delete removes a follower relationship from the database
func (s *FollowerStore) Delete(ctx context.Context, userId, followerId int64) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if userId == 0 {
		return errors.New("valid user_id is required")
	}
	if followerId == 0 {
		return errors.New("valid follower_id is required")
	}

	query := `DELETE FROM followers WHERE user_id = $1 AND follower_id = $2`
	result, err := s.db.ExecContext(ctx, query, userId, followerId)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("no follower relationship found with the given user_id and follower_id")
	}

	return nil
}

// GetByUserId retrieves all followers for a given user_id
func (s *FollowerStore) GetByUserId(ctx context.Context, userId int64) ([]*Follower, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if userId == 0 {
		return nil, errors.New("valid user_id is required")
	}

	var followers []*Follower
	query := `
		SELECT 
			f.user_id,
			f.follower_id,
			u1.first_name AS "user_first_name",
			u1.last_name AS "user_last_name",
			u1.avatar_url AS "user_avatar_url",
			u2.first_name AS "follower_first_name",
			u2.last_name AS "follower_last_name",
			u2.avatar_url AS "follower_avatar_url"
		FROM followers f
		LEFT JOIN users u1 ON f.user_id = u1.id
		LEFT JOIN users u2 ON f.follower_id = u2.id
		WHERE f.user_id = $1`

	type holder struct {
		userFirstName     string `db:"user_first_name"`
		userLastName      string `db:"user_last_name"`
		userAvatarUrl     string `db:"user_avatar_url"`
		followerFirstName string `db:"follower_first_name"`
		followerLastName  string `db:"follower_last_name"`
		followerAvatarUrl string `db:"follower_avatar_url"`
	}

	var temp holder
	err := s.db.QueryRowxContext(ctx, query, &followers).StructScan(&temp)
	if err != nil {
		return nil, err
	}

	return followers, nil
}

// GetByFollowerId retrieves all followees for a given follower_id
func (s *FollowerStore) GetByFollowerId(ctx context.Context, followerId int64) ([]*Follower, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if followerId == 0 {
		return nil, errors.New("valid follower_id is required")
	}

	var followers []*Follower
	query := `
		SELECT 
			f.user_id,
			f.follower_id,
			u1.id AS "user.id",
			u1.username AS "user.username",
			u2.id AS "follower.id",
			u2.username AS "follower.username"
		FROM followers f
		LEFT JOIN users u1 ON f.user_id = u1.id
		LEFT JOIN users u2 ON f.follower_id = u2.id
		WHERE f.follower_id = $1`

	err := s.db.SelectContext(ctx, &followers, query, followerId)
	if err != nil {
		return nil, err
	}

	return followers, nil
}

// GetAll retrieves all follower relationships
func (s *FollowerStore) GetAll(ctx context.Context) ([]*Follower, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var followers []*Follower
	query := `
		SELECT 
			f.user_id,
			f.follower_id,
			u1.id AS "user.id",
			u1.username AS "user.username",
			u2.id AS "follower.id",
			u2.username AS "follower.username"
		FROM followers f
		LEFT JOIN users u1 ON f.user_id = u1.id
		LEFT JOIN users u2 ON f.follower_id = u2.id`

	err := s.db.SelectContext(ctx, &followers, query)
	if err != nil {
		return nil, err
	}

	return followers, nil
}
