-- birdlens-be/cmd/migrate/migrations/000028_create_newsletter_updates_table.up.sql
CREATE TABLE IF NOT EXISTS newsletter_updates (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    commit_hash VARCHAR(40) UNIQUE NOT NULL,
    message TEXT NOT NULL,
    author VARCHAR(100) NOT NULL,
    committed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_processed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);