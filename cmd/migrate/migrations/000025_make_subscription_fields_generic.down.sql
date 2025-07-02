-- path: birdlens-be/cmd/migrate/migrations/000025_make_subscription_fields_generic.down.sql
-- Reverts the changes made in the up migration.
ALTER TABLE users RENAME COLUMN subscription_status TO stripe_subscription_status;
ALTER TABLE users RENAME COLUMN subscription_period_end TO stripe_subscription_period_end;

-- Re-add the columns that were dropped.
ALTER TABLE users ADD COLUMN stripe_subscription_id VARCHAR(255) NULL;
ALTER TABLE users ADD COLUMN stripe_price_id VARCHAR(255) NULL;
-- Logic: Also re-add the stripe_customer_id column.
ALTER TABLE users ADD COLUMN stripe_customer_id VARCHAR(255) NULL;