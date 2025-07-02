ALTER TABLE posts
-- Logic: To reverse the 'up' migration, we first drop the foreign key constraint
-- and then drop the entire column. The 'IF EXISTS' clause makes the script safe
-- to run even if parts of it have already been executed.
DROP CONSTRAINT IF EXISTS fk_posts_user_id,
DROP COLUMN IF EXISTS user_id;