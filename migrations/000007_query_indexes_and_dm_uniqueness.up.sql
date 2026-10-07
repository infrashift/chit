-- Indexes for queries the server actually runs, a uniqueness rule direct and
-- group channels never had, and four indexes that duplicated a UNIQUE
-- constraint's own.

BEGIN;

-- The search indexer pages posts by (update_at, id) every five seconds.
-- Without an index each tick scanned and sorted the whole table.
CREATE INDEX idx_posts_update_at_id ON posts (update_at, id);

-- The thread inbox lists a user's threads newest reply first.
CREATE INDEX idx_threads_last_reply_at ON threads (last_reply_at DESC);

-- DIRECT AND GROUP CHANNELS WERE NOT UNIQUE. They have no team, and the only
-- name index is UNIQUE (team_id, name), under which NULL team_ids are all
-- distinct. Two concurrent "open a DM" requests could each find no channel
-- and each create one; GetDirectChannelByName then returned either at random
-- and the conversation split between them.
--
-- Any duplicates already made are merged into the oldest before the index
-- can be built: its posts, threads and members move over, and the rest are
-- soft-deleted. Their counters are not summed; the next view recomputes them.
CREATE TEMP TABLE dm_merge AS
SELECT id AS dupe, keeper
FROM (
    SELECT id,
           FIRST_VALUE(id) OVER w AS keeper,
           ROW_NUMBER() OVER w AS rn
    FROM channels
    WHERE team_id IS NULL AND delete_at = 0
    WINDOW w AS (PARTITION BY name ORDER BY create_at, id)
) ranked
WHERE rn > 1;

UPDATE posts p SET channel_id = m.keeper FROM dm_merge m WHERE p.channel_id = m.dupe;
UPDATE threads t SET channel_id = m.keeper FROM dm_merge m WHERE t.channel_id = m.dupe;
INSERT INTO channel_members (channel_id, user_id, roles, last_viewed_at, msg_count, mention_count, notify_props, create_at)
    SELECT m.keeper, cm.user_id, cm.roles, cm.last_viewed_at, cm.msg_count, cm.mention_count, cm.notify_props, cm.create_at
    FROM channel_members cm JOIN dm_merge m ON cm.channel_id = m.dupe
    ON CONFLICT (channel_id, user_id) DO NOTHING;
UPDATE channels c
    SET delete_at = (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT,
        update_at = (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT
    FROM dm_merge m WHERE c.id = m.dupe;

DROP TABLE dm_merge;

CREATE UNIQUE INDEX idx_channels_direct_name_unique
    ON channels (name) WHERE team_id IS NULL AND delete_at = 0;

-- A UNIQUE constraint already builds its own index; these four were second
-- copies of the same thing, each paying its own cost on every write.
DROP INDEX IF EXISTS idx_users_kratos_id;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_teams_name;

COMMIT;
