package store

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type Location struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type LocationStore struct {
	db *sqlx.DB
}

func (s *LocationStore) GetByID(ctx context.Context, id int64) (*Location, error) {
	var location Location
	query := `SELECT id, code, name FROM location WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&location.ID,
		&location.Code,
		&location.Name,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return &location, nil
}
