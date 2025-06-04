package store

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type Bookmark struct {
	ID                int64      `db:"id"`
	UserID            int64      `db:"user_id"`
	HotspotLocationId string     `db:"hotspot_location_id"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         *time.Time `db:"updated_at"`
}

type BookmarksStore struct {
	db *sqlx.DB
}

func (store *BookmarksStore) Create(ctx context.Context, bookmark *Bookmark) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	query := `INSERT INTO bookmarks (user_id, hotspot_location_id, created_at, updated_at)
  VALUES (:user_id, :hotspot_location_id);
  `
	_, err := store.db.NamedExecContext(ctx, query, bookmark)
	if err != nil {
		return err
	}

	return nil
}

func (store *BookmarksStore) Delete(ctx context.Context, userID int64, hotspotLocationID int64) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	query := `DELETE FROM bookmarks WHERE user_id = :user_id AND hotspot_location_id = :hotspot_location_id`
	_, err := store.db.NamedExecContext(ctx, query, map[string]any{
		"user_id":             userID,
		"hotspot_location_id": hotspotLocationID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (store *BookmarksStore) GetByUserID(ctx context.Context, userID int64) ([]*Bookmark, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var bookmarks []*Bookmark
	query := `SELECT * FROM bookmarks WHERE user_id = :user_id`

	err := store.db.SelectContext(ctx, &bookmarks, query, map[string]any{
		"user_id": userID,
	})
	if err != nil {
		return nil, err
	}
	return bookmarks, nil
}

func (store *BookmarksStore) GetTrendingBookmarks(ctx context.Context, limit, offset int) (*PaginatedList[*Bookmark], error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var bookmarks []*Bookmark
	query := `SELECT * FROM bookmarks ORDER BY created_at DESC LIMIT :limit OFFSET :offset`
	err := store.db.SelectContext(ctx, &bookmarks, query, map[string]any{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		return nil, err
	}

	return &PaginatedList[*Bookmark]{Items: bookmarks}, nil
}

func (store *BookmarksStore) Exists(ctx context.Context, userID int64, hotspotLocationID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	query := `SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id=$1 AND hotspot_location_id=$2);`
	var exists bool

	err := store.db.GetContext(ctx, &exists, query, userID, hotspotLocationID)
	if err != nil {
		log.Printf("Error checking bookmark existence: %v", err)
		return false, err
	}

	return exists, nil
}
