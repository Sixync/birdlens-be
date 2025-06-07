package database

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/lib/pq"
)

const defaultTimeout = 3 * time.Second

func New(dbConn string) (*sqlx.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	db, err := sqlx.ConnectContext(ctx, "postgres", dbConn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(2 * time.Hour)

	return db, nil
}

// Ping checks the database connection.
