package store

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// Post represents a post entity
type Post struct {
	Id            int64      `json:"id" db:"id"`
	Content       string     `json:"content" db:"content"`
	LocationName  string     `json:"location_name" db:"location_name"`
	Latitude      float64    `json:"latitude" db:"latitude"`
	Longitude     float64    `json:"longitude" db:"longitude"`
	PrivacyLevel  string     `json:"privacy_level" db:"privacy_level"`
	Type          string     `json:"type" db:"type"`
	IsFeatured    bool       `json:"is_featured" db:"is_featured"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at" db:"updated_at"`
	ReactionCount int        `json:"reaction_count" db:"reaction_count"`
	UserId        int64      `json:"user_id" db:"user_id"`
}

type PostReaction struct {
	UserId       int64  `json:"user_id" db:"user_id"`
	PostId       int64  `json:"post_id" db:"post_id"`
	ReactionType string `json:"reaction_type" db:"reaction_type"`
}

type Media struct {
	Id        int64     `json:"id" db:"id"`
	PostId    int64     `json:"post_id" db:"post_id"`
	MediaUrl  string    `json:"media_url" db:"media_url"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
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

	query := `
    INSERT INTO posts (user_id, content, location_name, latitude, longitude, privacy_level, type, is_featured)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING id, created_at`

	err := s.db.QueryRowContext(ctx, query, post.UserId, post.Content, post.LocationName, post.Latitude, post.Longitude, post.PrivacyLevel, post.Type, post.IsFeatured).Scan(&post.Id, &post.CreatedAt)
	if err != nil {
		log.Println("Create post error", err)
		return err
	}

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
	query := `
    SELECT id, location_name, latitude, longitude, privacy_level, type, is_featured, created_at, updated_at
    FROM posts WHERE id = $1;
  `
	err := s.db.QueryRowContext(ctx, query, id).Scan(&post.Id, &post.LocationName, &post.Latitude, &post.Longitude, &post.PrivacyLevel, &post.Type, &post.IsFeatured, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, sql.ErrNoRows
		default:
			return nil, err
		}
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

func (s *PostStore) GetLikeCounts(ctx context.Context, postId int64) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var count int
	query := `
    SELECT COUNT(*) 
    FROM post_reactions 
    WHERE post_id = $1 AND reaction_type = 'like'
  `
	err := s.db.GetContext(ctx, &count, query, postId)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *PostStore) GetCommentCounts(ctx context.Context, postId int64) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var count int
	query := `
    SELECT COUNT(*) 
    FROM comments 
    WHERE post_id = $1
  `
	err := s.db.GetContext(ctx, &count, query, postId)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *PostStore) GetMediaUrlsById(ctx context.Context, postId int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var mediaUrls []string
	query := `
    SELECT media_url
    FROM media
    WHERE post_id = $1
  `
	err := s.db.SelectContext(ctx, &mediaUrls, query, postId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No media found for this post
		}
		return nil, err
	}

	return mediaUrls, nil
}

func (s *PostStore) AddUserReaction(ctx context.Context, userId, postId int64, reactionType string) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
    INSERT INTO post_reactions (user_id, post_id, reaction_type)
    VALUES ($1, $2, $3)
    ON CONFLICT (user_id, post_id) DO UPDATE SET reaction_type = EXCLUDED.reaction_type
  `
	_, err := s.db.ExecContext(ctx, query, userId, postId, reactionType)
	if err != nil {
		return err
	}

	return nil
}

func (s *PostStore) UserLiked(ctx context.Context, userId, postId int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
    SELECT EXISTS (
      SELECT 1 
      FROM post_reactions 
      WHERE user_id = $1 AND post_id = $2
    )
  `
	var exists bool
	err := s.db.GetContext(ctx, &exists, query, userId, postId)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *PostStore) AddMediaUrl(ctx context.Context, postId int64, mediaUrl string) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `
    INSERT INTO media (post_id, media_url)
    VALUES ($1, $2)
    ON CONFLICT DO NOTHING
  `
	_, err := s.db.ExecContext(ctx, query, postId, mediaUrl)
	if err != nil {
		return err
	}

	return nil
}

func (s *PostStore) GetTrendingPosts(ctx context.Context, duration time.Time, limit, offset int) (*PaginatedList[*Post], error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if limit <= 0 || limit > 100 {
		return nil, errors.New("limit must be between 1 and 100")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}

	var totalCount int64
	countQuery := `
    SELECT COUNT(*) 
    FROM posts 
    WHERE created_at >= $1`
	err := s.db.GetContext(ctx, &totalCount, countQuery, duration)
	if err != nil {
		return nil, err
	}

	var posts []*Post
	// Logic: The p.user_id column was missing from the SELECT and GROUP BY clauses.
	// This caused the post struct to have a user ID of 0, leading to a "no rows in result set"
	// error when fetching the user details in the handler. It has now been added.
	query := `
    SELECT 
        p.id, 
        p.user_id,
        p.content, 
        p.location_name, 
        p.latitude, 
        p.longitude, 
        p.privacy_level, 
        p.type, 
        p.is_featured, 
        p.created_at, 
        p.updated_at,
        COALESCE(COUNT(pr.reaction_type), 0) AS reaction_count
    FROM posts p 
    LEFT JOIN post_reactions pr ON p.id = pr.post_id 
    WHERE p.created_at >= $1 
    GROUP BY p.id, p.user_id, p.content, p.location_name, p.latitude, p.longitude, p.privacy_level, p.type, p.is_featured, p.created_at, p.updated_at
    ORDER BY reaction_count DESC
    LIMIT $2 OFFSET $3;
  `
	err = s.db.SelectContext(ctx, &posts, query, duration, limit, offset)
	if err != nil {
		return nil, err
	}

	return NewPaginatedList(posts, int(totalCount), limit, offset)
}

func (s *PostStore) GetFollowerPosts(ctx context.Context, userId int64, limit, offset int) (*PaginatedList[*Post], error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if limit <= 0 || limit > 100 {
		return nil, errors.New("limit must be between 1 and 100")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}

	var totalCount int64
	countQuery := `
    SELECT COUNT(*) 
    FROM posts p 
    JOIN followers f ON p.user_id = f.follower_id
    WHERE f.user_id = $1;
  `
	err := s.db.GetContext(ctx, &totalCount, countQuery, userId)
	if err != nil {
		return nil, err
	}

	var posts []*Post
	query := `
    SELECT p.id, p.content, p.location_name, p.latitude, p.longitude, p.privacy_level, p.type, p.is_featured, p.created_at, p.updated_at, p.user_id 
    FROM posts p 
    JOIN followers f ON p.user_id = f.follower_id 
    WHERE f.user_id = $1 
    ORDER BY p.created_at DESC 
    LIMIT $2 OFFSET $3;
  `

	err = s.db.SelectContext(ctx, &posts, query, userId, limit, offset)
	if err != nil {
		return nil, err
	}

	return NewPaginatedList(posts, int(totalCount), limit, offset)
}