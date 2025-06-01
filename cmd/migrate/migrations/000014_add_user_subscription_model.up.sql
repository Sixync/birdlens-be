-- Create the subscriptions table if it doesn't exist
CREATE TABLE subscriptions (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, -- Use SERIAL or BIGSERIAL for PostgreSQL AUTOINCREMENT
  name TEXT NOT NULL,
  description TEXT,
  price DECIMAL(10, 2) NOT NULL,
  duration_days INT NOT NULL,
  created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP(0) WITH TIME ZONE
);

-- Add the subscription_id column to the users table
-- We assume users table already exists.
-- The type (BIGINT) must match the primary key type of the subscriptions table.
ALTER TABLE users
ADD COLUMN subscription_id BIGINT;

-- Add the foreign key constraint
-- This links subscription_id in 'users' to id in 'subscriptions'.
ALTER TABLE users
ADD CONSTRAINT fk_users_subscription
FOREIGN KEY (subscription_id)
REFERENCES subscriptions (id)
ON DELETE SET NULL; -- OR ON DELETE CASCADE OR ON DELETE RESTRICT/NO ACTION
