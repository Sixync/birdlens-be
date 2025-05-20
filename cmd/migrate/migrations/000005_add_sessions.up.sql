CREATE TABLE IF NOT EXISTS sessions (
    id BIGINT NOT NULL,
    user_email VARCHAR(255) NOT NULL,
    refresh_token VARCHAR(255) NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp(0) with time zone,
    PRIMARY KEY (id),
    FOREIGN KEY (id) REFERENCES users (id) ON DELETE CASCADE
);
