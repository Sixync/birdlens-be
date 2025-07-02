-- birdlens-be/cmd/migrate/migrations/000022_seed_roles.up.sql

-- Logic: Seed the 'roles' table with essential roles that the application depends on.
-- The ON CONFLICT DO NOTHING clause ensures that if the roles already exist (e.g., from a manual insert or a previous run),
-- the command will not fail, making the migration safe to re-run (idempotent).
INSERT INTO roles (name) VALUES ('admin'), ('user') ON CONFLICT (name) DO NOTHING;