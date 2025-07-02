-- path: birdlens-be/cmd/migrate/migrations/000024_add_unique_constraint_to_roles_name.up.sql
-- Adds a UNIQUE constraint to the name column of the roles table.
-- Giving the constraint a specific name (roles_name_key) is good practice.
ALTER TABLE roles
ADD CONSTRAINT roles_name_unique UNIQUE (name);