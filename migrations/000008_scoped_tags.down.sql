-- Back to one global Tag vocabulary. LOSSY: private Tags and channel_tags
-- are dropped, applied_by is forgotten, and channel Tags that share a name
-- (in different channels) are merged into the oldest by id. The chit-system
-- user stays, since posts may not reference it but nothing else is lost by
-- keeping it.

BEGIN;

DROP TABLE channel_tags;

DELETE FROM tags WHERE scope = 'private';

CREATE TEMP TABLE tag_merge AS
SELECT id AS dupe, keeper
FROM (
    SELECT id, FIRST_VALUE(id) OVER (PARTITION BY name ORDER BY id) AS keeper
    FROM tags
) t
WHERE id <> keeper;

INSERT INTO message_tags (message_id, tag_id)
SELECT mt.message_id, m.keeper
FROM message_tags mt JOIN tag_merge m ON m.dupe = mt.tag_id
ON CONFLICT DO NOTHING;
DELETE FROM tags WHERE id IN (SELECT dupe FROM tag_merge);
DROP TABLE tag_merge;

DROP INDEX idx_tags_private_name;
DROP INDEX idx_tags_channel_name;
DROP INDEX idx_tags_owner_id;
ALTER TABLE tags
    DROP CONSTRAINT tags_scope_channel_check,
    DROP CONSTRAINT tags_scope_check,
    DROP COLUMN name_key,
    DROP COLUMN owner_id,
    DROP COLUMN scope,
    DROP COLUMN channel_id,
    DROP COLUMN create_at,
    DROP COLUMN update_at;
ALTER TABLE tags ADD CONSTRAINT tags_name_key UNIQUE (name);

ALTER TABLE message_tags
    DROP COLUMN applied_by,
    DROP COLUMN create_at;

COMMIT;
