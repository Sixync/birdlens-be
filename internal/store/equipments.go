package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Equipment struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	ImageUrl    *string `json:"thumbnail_url"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type EquipmentStore struct {
	db *sqlx.DB
}

func (s *EquipmentStore) GetByID(ctx context.Context, id int64) (*Equipment, error) {
	var equipment Equipment
	query := `SELECT id, name, description, price, thumbnail_url, created_at, updated_at FROM equipments WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&equipment.ID,
		&equipment.Name,
		&equipment.Description,
		&equipment.Price,
		&equipment.ImageUrl,
		&equipment.CreatedAt,
		&equipment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &equipment, nil
}
