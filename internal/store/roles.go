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
	roleIdQuery := "SELECT id FROM roles WHERE name = $1;"
	var roleID int64
	err := s.db.GetContext(ctx, &roleID, roleIdQuery, roleName)
	if err != nil {
		return err
	}

	query := "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2);"
	_, err = s.db.ExecContext(ctx, query, userID, roleID)
	if err != nil {
		return err
	}
	return nil
}

// GetUserRoles retrieves all role names for a specific user.
func (s *RoleStore) GetUserRoles(ctx context.Context, userID int64) ([]string, error) {
    var roleNames []string
    query := `SELECT r.name FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = $1`
    err := s.db.SelectContext(ctx, &roleNames, query, userID)
    if err != nil {
        return nil, err
    }
    return roleNames, nil
}