-- Reverts the addition of sighting-related columns from the posts table.
ALTER TABLE posts
DROP COLUMN IF EXISTS sighting_date,
DROP COLUMN IF EXISTS tagged_species_code;