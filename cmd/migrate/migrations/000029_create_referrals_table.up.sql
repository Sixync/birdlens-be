-- Creates the referrals table to track user-to-user referrals.
CREATE TABLE IF NOT EXISTS referrals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    referrer_id BIGINT NOT NULL,
    referee_id BIGINT UNIQUE NOT NULL, -- The new user, can only be referred once.
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- e.g., 'pending', 'completed'
    referral_code_used VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_referrals_referrer FOREIGN KEY (referrer_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_referrals_referee FOREIGN KEY (referee_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Add an index for faster lookups on the referee_id
CREATE INDEX IF NOT EXISTS idx_referrals_referee_id ON referrals(referee_id);