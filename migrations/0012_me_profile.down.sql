-- 0012_me_profile.down.sql
-- ====================================================================
ALTER TABLE sessions DROP COLUMN IF EXISTS last_seen_at;
DROP TABLE IF EXISTS user_avatars;
