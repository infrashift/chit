-- Duplicate direct channels merged by the up migration are not split apart
-- again: which posts belonged to which copy is not recorded.
BEGIN;

DROP INDEX IF EXISTS idx_channels_direct_name_unique;
DROP INDEX IF EXISTS idx_threads_last_reply_at;
DROP INDEX IF EXISTS idx_posts_update_at_id;

CREATE INDEX IF NOT EXISTS idx_users_kratos_id ON users (kratos_id);
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_teams_name ON teams (name);

COMMIT;
