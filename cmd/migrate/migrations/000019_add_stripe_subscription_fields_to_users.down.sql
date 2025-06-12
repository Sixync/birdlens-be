
ALTER TABLE users
DROP COLUMN IF EXISTS stripe_customer_id,
DROP COLUMN IF EXISTS stripe_subscription_id,
DROP COLUMN IF EXISTS stripe_price_id,
DROP COLUMN IF EXISTS stripe_subscription_status,
DROP COLUMN IF EXISTS stripe_subscription_period_end;