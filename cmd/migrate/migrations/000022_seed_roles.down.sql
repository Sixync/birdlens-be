-- birdlens-be/cmd/migrate/migrations/000022_seed_roles.down.sql

-- Logic: The 'down' migration should reverse the 'up' migration by deleting the seeded roles.
DELETE FROM roles WHERE name IN ('admin', 'user');