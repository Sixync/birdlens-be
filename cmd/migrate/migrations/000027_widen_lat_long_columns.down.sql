-- Reverts the precision change for latitude and longitude columns.
ALTER TABLE posts
    ALTER COLUMN latitude TYPE DECIMAL(10, 8),
    ALTER COLUMN longitude TYPE DECIMAL(10, 8);