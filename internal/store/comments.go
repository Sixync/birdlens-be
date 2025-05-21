package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type Comment struct {
	ID        int64      `json:"id"`
	PostID    int64      `json:"post_id"`
	UserID    int64      `json:"user_id"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type CommentStore struct {
	db *sqlx.DB
}

func (s *CommentStore) GetById(ctx context.Context, commentId int64) (*Comment, error) {
	// Implement the logic to get a comment by ID from the database
	query := `SELECT id, post_id, user_id, content, created_at, updated_at 
        FROM comments WHERE id = $1`
	var comment Comment
	err := s.db.QueryRowContext(ctx, query, commentId).Scan(&comment.ID, &comment.PostID,
		&comment.UserID, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, sql.ErrNoRows
		default:
			return nil, err
		}
	}
	return &comment, nil
}

func (s *CommentStore) Create(ctx context.Context, comment *Comment) error {
	// Implement the logic to create a new comment in the database
	query := `INSERT INTO comments (post_id, user_id, content) 
        VALUES ($1, $2, $3) RETURNING id, created_at;`
	err := s.db.QueryRowContext(ctx, query, comment.PostID, comment.UserID,
		comment.Content).Scan(&comment.ID, &comment.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *CommentStore) Update(ctx context.Context, comment *Comment) error {
	// Implement the logic to update an existing comment in the database
	query := `UPDATE comments SET content = $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, comment.Content, time.Now(), comment.ID)
	if err != nil {
		return err
	}
	return nil
}

func (s *CommentStore) Delete(ctx context.Context, commentId int64) error {
	// Implement the logic to delete a comment from the database
	query := `DELETE FROM comments WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, commentId)
	if err != nil {
		return err
	}
	return nil
}

func (s *CommentStore) GetByPostId(ctx context.Context, postId int64, limit, offset int) (*PaginatedList[*Comment], error) {
	// Implement the logic to get comments by post ID with pagination
	query := `SELECT id, post_id, user_id, content, created_at, updated_at 
        FROM comments WHERE post_id = $1 LIMIT $2 OFFSET $3`
	rows, err := s.db.QueryContext(ctx, query, postId, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.PostID,
			&comment.UserID, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, &comment)
	}

	return &PaginatedList[*Comment]{Items: comments}, nil
}
