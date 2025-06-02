package store

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type Event struct {
	ID            int64      `json:"id" db:"id"`
	Title         string     `json:"title" db:"title"`
	Description   string     `json:"description" db:"description"`
	CoverPhotoUrl *string    `json:"cover_photo_url,omitempty" db:"cover_photo_url"`
	StartDate     time.Time  `json:"start_date" db:"start_date"`
	EndDate       time.Time  `json:"end_date" db:"end_date"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at" db:"updated_at"`
}

type EventStore struct {
	db *sqlx.DB
}

func (s *EventStore) GetByID(ctx context.Context, id int64) (*Event, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var event Event
	log.Println("GetByID event id", id)

	query := `
  SELECT * FROM events
  WHERE id = $1
  `
	err := s.db.GetContext(ctx, &event, query, id)
	if err != nil {
		return nil, err
	}

	log.Println("GetByID event", event)
	return &event, nil
}

func (s *EventStore) GetAll(ctx context.Context, limit, offset int) (*PaginatedList[*Event], error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var events []*Event

	query := `
    SELECT id, title, description, cover_photo_url, start_date, end_date, created_at, updated_at
    FROM events
    ORDER BY created_at DESC
    LIMIT $1 OFFSET $2
  `

	err := s.db.SelectContext(ctx, &events, query, limit, offset)
	if err != nil {
		log.Println("GetAll events error", err)
		return nil, err
	}

	countQuery := `SELECT COUNT(*) FROM events`
	var totalCount int
	err = s.db.GetContext(ctx, &totalCount, countQuery)
	if err != nil {
		log.Println("GetAll total count error", err)
		return nil, err
	}

	return NewPaginatedList(events, totalCount, limit, offset)
}

func (s *EventStore) Create(ctx context.Context, event *Event) error {
	query := `
    INSERT INTO events (title, description, start_date, end_date)
    VALUES (:title, :description, :start_date, :end_date)
  `
	result, err := s.db.NamedExecContext(ctx, query, event)
	if err != nil {
		log.Println("Create event error", err)
		return err
	}

	log.Println("the result is", result)

	log.Println("Create event id", event.ID)
	return nil
}

func (s *EventStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM events WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		log.Println("Delete event error", err)
		return err
	}

	log.Println("Delete event id", id)
	return nil
}
