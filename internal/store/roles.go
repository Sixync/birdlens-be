package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Role struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type RoleStore struct {
	db *sqlx.DB
}

var (
	ADMIN string = "admin"
	USER  string = "user"
)

func (s *RoleStore) GetByID(ctx context.Context, id int64) (*Role, error) {
	role := &Role{}
	query := "SELECT id, name FROM roles WHERE id = ?"
	err := s.db.GetContext(ctx, role, query, id)
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (s *RoleStore) AddUserToRole(ctx context.Context, userID int64, roleName string) error {
	roleIdQuery := "SELECT id FROM roles WHERE name = ?"
	var roleID int64
	err := s.db.GetContext(ctx, &roleID, roleIdQuery, roleName)
	if err != nil {
		return err
	}

	query := "INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)"
	_, err = s.db.ExecContext(ctx, query, userID, roleID)
	if err != nil {
		return err
	}
	return nil
}
