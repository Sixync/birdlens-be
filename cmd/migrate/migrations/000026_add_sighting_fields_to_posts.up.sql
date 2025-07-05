-- Adds columns to support the 'sighting' post type.
ALTER TABLE posts
ADD COLUMN sighting_date TIMESTAMP WITH TIME ZONE,
ADD COLUMN tagged_species_code VARCHAR(20);