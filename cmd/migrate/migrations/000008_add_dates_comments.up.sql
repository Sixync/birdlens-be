ALTER TABLE comments
ADD COLUMN created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
ADD COLUMN updated_at timestamp(0) with time zone;
