ALTER TABLE users
ADD COLUMN stripe_customer_id VARCHAR(255) NULL,
ADD COLUMN stripe_subscription_id VARCHAR(255) NULL,
ADD COLUMN stripe_price_id VARCHAR(255) NULL,
ADD COLUMN stripe_subscription_status VARCHAR(50) NULL,
ADD COLUMN stripe_subscription_period_end TIMESTAMP WITH TIME ZONE NULL;