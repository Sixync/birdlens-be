ALTER TABLE users
ADD COLUMN forgot_password_token VARCHAR(255) NULL,
ADD COLUMN forgot_password_token_expires_at TIMESTAMP NULL;
