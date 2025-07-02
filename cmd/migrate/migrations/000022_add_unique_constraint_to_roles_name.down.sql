-- path: birdlens-be/cmd/migrate/migrations/000024_add_unique_constraint_to_roles_name.down.sql
-- Reverts the change by dropping the unique constraint.
ALTER TABLE roles
DROP CONSTRAINT roles_name_unique;