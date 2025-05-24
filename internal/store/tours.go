package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type Tour struct {
	ID           int64      `json:"id"`
	EventId      int64      `json:"event_id"`
	Event        *Event     `json:"event"`
	Price        float64    `json:"price"`
	Capacity     int        `json:"capacity"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	ThumbnailUrl *string    `json:"thumbnail_url"`
	Duration     int        `json:"duration"`
	StartDate    string     `json:"start_date"`
	EndDate      string     `json:"end_date"`
	LocationId   int64      `json:"location_id"`
	Location     *Location  `json:"location"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
	// ImagesUrl []TourImages
}

type TourStore struct {
	db *sqlx.DB
}

func (s *TourStore) Create(ctx context.Context, tour *Tour) error {
	query := `INSERT INTO tours (event_id, name, description, thumbnail_url, price, duration, start_date, end_date, location_id, created_at) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()) RETURNING id`
	err := s.db.QueryRowContext(ctx, query,
		tour.EventId,
		tour.Name,
		tour.Description,
		tour.ThumbnailUrl,
		tour.Price,
		tour.Duration,
		tour.StartDate,
		tour.EndDate,
		tour.LocationId).Scan(&tour.ID)
	return err
}

func (s *TourStore) GetByID(ctx context.Context, id int64) (*Tour, error) {
	query := `SELECT id, event_id, name, description, thumbnail_url, price, duration, start_date, end_date, location_id, created_at, updated_at
        FROM tours WHERE id = $1`

	tour := &Tour{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&tour.ID,
		&tour.EventId,
		&tour.Name,
		&tour.Description,
		&tour.ThumbnailUrl,
		&tour.Price,
		&tour.Duration,
		&tour.StartDate,
		&tour.EndDate,
		&tour.LocationId,
		&tour.CreatedAt,
		&tour.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return tour, nil
}

func (s *TourStore) Update(ctx context.Context, tour *Tour) error {
	query := `UPDATE tours SET event_id = $1, name = $2, description = $3, thumbnail_url = $4, price = $5, duration = $6, start_date = $7, end_date = $8, location_id = $9, updated_at = NOW() WHERE id = $10`
	_, err := s.db.ExecContext(ctx, query,
		tour.EventId,
		tour.Name,
		tour.Description,
		tour.ThumbnailUrl,
		tour.Price,
		tour.Duration,
		tour.StartDate,
		tour.EndDate,
		tour.LocationId,
		tour.ID)
	return err
}

func (s *TourStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM tours WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *TourStore) GetAll(ctx context.Context, limit, offset int) (*PaginatedList[*Tour], error) {
	query := `SELECT id, name, description, thumbnail_url, price, duration, start_date, end_date, location_id, created_at, updated_at
        FROM tours ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var tours []*Tour
	for rows.Next() {
		tour := &Tour{}
		err := rows.Scan(&tour.ID, &tour.Name, &tour.Description, &tour.ThumbnailUrl, &tour.Price, &tour.Duration, &tour.StartDate, &tour.EndDate, &tour.LocationId, &tour.CreatedAt, &tour.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tours = append(tours, tour)
	}
	paginatedList, err := NewPaginatedList(tours, len(tours), limit, offset)
	if err != nil {
		return nil, err
	}
	return paginatedList, nil
}
