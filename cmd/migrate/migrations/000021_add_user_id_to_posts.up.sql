ALTER TABLE posts
ADD COLUMN user_id BIGINT;

-- Logic: First, update any existing posts that might have a NULL user_id.
-- We'll set them to a default user (e.g., user with ID 1, assuming it's an admin or system user).
-- This step is crucial to avoid errors when adding the NOT NULL constraint on a table with existing data.
-- In a fresh database, this UPDATE will do nothing, which is fine.
UPDATE posts SET user_id = 1 WHERE user_id IS NULL;

-- Logic: Now that all rows are guaranteed to have a non-null user_id,
-- alter the column to enforce this rule for all future inserts and updates.
-- Also add a foreign key to ensure data integrity.
ALTER TABLE posts
ALTER COLUMN user_id SET NOT NULL,
ADD CONSTRAINT fk_posts_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;