package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// Post represents a post entity
type Post struct {
	Id           int64      `json:"id" db:"id"`
	LocationName string     `json:"location_name" db:"location_name"`
	Latitude     float64    `json:"latitude" db:"latitude"`
	Longitude    float64    `json:"longitude" db:"longitude"`
	PrivacyLevel string     `json:"privacy_level" db:"privacy_level"`
	Type         string     `json:"type" db:"type"`
	IsFeatured   bool       `json:"is_featured" db:"is_featured"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at" db:"updated_at"`
}

// PostStore handles database operations for posts
type PostStore struct {
	db *sqlx.DB
}

// NewPostStore creates a new PostStore instance
func NewPostStore(db *sqlx.DB) *PostStore {
	return &PostStore{db: db}
}

// Create inserts a new post into the database
func (s *PostStore) Create(ctx context.Context, post *Post) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if post == nil {
		return errors.New("post cannot be nil")
	}

	query := `
		INSERT INTO posts (
			location_name, latitude, longitude, 
			privacy_level, type, is_featured
		) VALUES (
			:group_id, :location_name, :latitude, :longitude, 
			:privacy_level, :type, :is_featured
		) RETURNING id, created_at, updated_at`

	// Use a temporary struct to capture returned fields
	type result struct {
		Id        int64      `db:"id"`
		CreatedAt time.Time  `db:"created_at"`
		UpdatedAt *time.Time `db:"updated_at"`
	}

	var res result
	err := s.db.QueryRowxContext(ctx, query, post).StructScan(&res)
	if err != nil {
		return err
	}

	post.Id = res.Id
	post.CreatedAt = res.CreatedAt
	post.UpdatedAt = res.UpdatedAt
	return nil
}

// Update modifies an existing post in the database
func (s *PostStore) Update(ctx context.Context, post *Post) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if post == nil {
		return errors.New("post cannot be nil")
	}
	if post.Id == 0 {
		return errors.New("post ID is required for update")
	}

	query := `
		UPDATE posts 
		SET 
			location_name = :location_name,
			latitude = :latitude,
			longitude = :longitude,
			privacy_level = :privacy_level,
			type = :type,
			is_featured = :is_featured
		WHERE id = :id
		RETURNING created_at, updated_at`

	// Use a temporary struct to capture returned fields
	type result struct {
		CreatedAt time.Time  `db:"created_at"`
		UpdatedAt *time.Time `db:"updated_at"`
	}

	var res result
	err := s.db.QueryRowxContext(ctx, query, post).StructScan(&res)
	if err == sql.ErrNoRows {
		return errors.New("no post found with the given ID")
	}
	if err != nil {
		return err
	}

	post.CreatedAt = res.CreatedAt
	post.UpdatedAt = res.UpdatedAt
	return nil
}

// Delete removes a post from the database by ID
func (s *PostStore) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if id == 0 {
		return errors.New("valid post ID is required")
	}

	query := `DELETE FROM posts WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("no post found with the given ID")
	}
	return nil
}

// GetById retrieves a post by ID
func (s *PostStore) GetById(ctx context.Context, id int64) (*Post, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if id == 0 {
		return nil, errors.New("valid post ID is required")
	}

	var post Post
	query := `SELECT * FROM posts WHERE id = ?`
	err := s.db.GetContext(ctx, &post, query, id)
	if err == sql.ErrNoRows {
		return nil, errors.New("no post found with the given ID")
	}
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// GetAll retrieves all posts with pagination
func (s *PostStore) GetAll(ctx context.Context, limit, offset int) (*PaginatedList[*Post], error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if limit <= 0 || limit > 100 {
		return nil, errors.New("limit must be between 1 and 100")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}

	// Query total count
	var totalCount int64
	countQuery := `SELECT COUNT(*) FROM posts`
	err := s.db.GetContext(ctx, &totalCount, countQuery)
	if err != nil {
		return nil, err
	}

	// Query paginated posts
	var posts []*Post
	query := `
		SELECT * FROM posts 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`
	err = s.db.SelectContext(ctx, &posts, query, limit, offset)
	if err != nil {
		return nil, err
	}

	return NewPaginatedList(posts, int(totalCount), limit, offset)
}
