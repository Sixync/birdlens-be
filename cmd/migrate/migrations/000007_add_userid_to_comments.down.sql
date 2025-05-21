ALTER TABLE comments
DROP COLUMN user_id,
DROP CONSTRAINT fk_comments_user_id;
