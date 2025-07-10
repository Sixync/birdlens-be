-- Creates the notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL, -- The user who will receive the notification
    type VARCHAR(50) NOT NULL, -- e.g., 'referral_success', 'new_follower'
    message TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Add an index for faster lookups on the user_id
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);