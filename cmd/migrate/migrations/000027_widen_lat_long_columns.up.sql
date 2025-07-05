-- Corrects the precision for latitude and longitude columns to prevent overflow.
ALTER TABLE posts
    ALTER COLUMN latitude TYPE DECIMAL(10, 7),
    ALTER COLUMN longitude TYPE DECIMAL(11, 8);