package store

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type Event struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	CoverPhotoUrl *string    `json:"cover_photo_url"`
	StartDate     time.Time  `json:"start_date"`
	EndDate       time.Time  `json:"end_date"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

type EventStore struct {
	db *sqlx.DB
}

func (s *EventStore) GetByID(ctx context.Context, id int64) (*Event, error) {
	var event Event
	log.Println("GetByID event id", id)
	query := `SELECT id, title, description, cover_photo_url, start_date, end_date, created_at, updated_at
        FROM events WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.CoverPhotoUrl,
		&event.StartDate,
		&event.EndDate,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil && err != sql.ErrNoRows {
		log.Println("GetByID event error", err)
		return nil, err
	}

	log.Println("GetByID event", event)
	log.Println("GetByID test no row", err)
	return &event, nil
}
