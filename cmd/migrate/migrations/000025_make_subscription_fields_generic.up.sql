-- path: birdlens-be/cmd/migrate/migrations/000025_make_subscription_fields_generic.up.sql
-- Renames stripe-specific subscription columns to be generic.
ALTER TABLE users RENAME COLUMN stripe_subscription_status TO subscription_status;
ALTER TABLE users RENAME COLUMN stripe_subscription_period_end TO subscription_period_end;

-- These columns are not needed for the manual renewal flow and can be dropped
-- to simplify the model. They are specific to Stripe's recurring billing APIs.
ALTER TABLE users DROP COLUMN IF EXISTS stripe_subscription_id;
ALTER TABLE users DROP COLUMN IF EXISTS stripe_price_id;

-- Logic: Also remove the stripe_customer_id as it's no longer used.
ALTER TABLE users DROP COLUMN IF EXISTS stripe_customer_id;