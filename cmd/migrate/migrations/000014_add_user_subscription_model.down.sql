ALTER TABLE users
DROP CONSTRAINT IF EXISTS fk_users_subscription; -- IF EXISTS is good for idempotency

-- 2. Drop the subscription_id column from the users table
-- NOTE: Dropping columns can be destructive to data.
-- In production, you might soft-delete or archive data before dropping.
ALTER TABLE users
DROP COLUMN IF EXISTS subscription_id; -- IF EXISTS is good for idempotency

-- 3. Drop the subscriptions table
-- This will delete all data in the subscriptions table.
DROP TABLE IF EXISTS subscriptions; -- IF EXISTS is good for idempotency
