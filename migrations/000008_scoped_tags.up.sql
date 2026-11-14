-- Scoped Tags (P-CHIT-7, HALO ADR-0009).
--
-- A Tag now has an owner and a scope. A private Tag is its owner's alone; a
-- channel Tag belongs to one channel, whose members see and apply it there.
-- A name is unique per owner (private) or per channel (channel) by its key,
-- the name after NFKC normalization and case folding, instead of unique on
-- the whole server. Tags can be applied to channels too (channel_tags), and
-- each application records who applied it.
--
-- EXISTING TAGS were one global vocabulary, applied to posts by anyone, with
-- no record of who created or applied them. Each one becomes a channel Tag
-- in every channel where a post carries it, so every member keeps seeing
-- what they saw. Its owner is the system actor "chit-system"
-- (model.SystemActorID), because the creator is not known; a system_admin
-- may rename or delete Tags that the system actor owns. A Tag applied
-- nowhere is dropped. The first channel (by id) keeps the Tag's id, so a
-- client's cached id still works there; the other channels get new ids.
--
-- name_key here is lower(normalize(name, NFKC)). The server computes it
-- with Unicode case folding, which differs from lower() for a few letters
-- (ß folds to ss); the old names were unique server-wide, so no channel can
-- hold two migrated Tags with one key, and the server rewrites the key when
-- a Tag is renamed.

BEGIN;

ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_name_key;
ALTER TABLE tags
    ADD COLUMN name_key   VARCHAR(200),
    ADD COLUMN owner_id   UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN scope      VARCHAR(10),
    ADD COLUMN channel_id UUID REFERENCES channels(id) ON DELETE CASCADE,
    ADD COLUMN create_at  BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN update_at  BIGINT NOT NULL DEFAULT 0;

ALTER TABLE message_tags
    ADD COLUMN applied_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN create_at  BIGINT NOT NULL DEFAULT 0;

-- The system actor, added only when there is something for it to own.
INSERT INTO users (id, kratos_id, username, display_name, email, roles, actor_type, create_at, update_at)
SELECT '00000000-0000-7000-8000-000000000001', NULL, 'chit-system', 'Chit', NULL, 'system_user', 'bot',
       (extract(epoch FROM now()) * 1000)::bigint, (extract(epoch FROM now()) * 1000)::bigint
WHERE EXISTS (SELECT 1 FROM message_tags)
ON CONFLICT (id) DO NOTHING;

-- One row per (old tag, channel it is used in). The first channel keeps the
-- old id.
CREATE TEMP TABLE tag_split AS
SELECT old_id, channel_id,
       CASE WHEN ROW_NUMBER() OVER (PARTITION BY old_id ORDER BY channel_id) = 1
            THEN old_id ELSE gen_random_uuid() END AS new_id
FROM (
    SELECT DISTINCT mt.tag_id AS old_id, p.channel_id
    FROM message_tags mt
    JOIN posts p ON p.id = mt.message_id
) used;

-- The copies for the second and later channels.
INSERT INTO tags (id, name)
SELECT s.new_id, t.name
FROM tag_split s JOIN tags t ON t.id = s.old_id
WHERE s.new_id <> s.old_id;

UPDATE tags t
SET owner_id   = '00000000-0000-7000-8000-000000000001',
    scope      = 'channel',
    channel_id = s.channel_id,
    name_key   = lower(normalize(t.name, NFKC)),
    create_at  = (extract(epoch FROM now()) * 1000)::bigint,
    update_at  = (extract(epoch FROM now()) * 1000)::bigint
FROM tag_split s
WHERE t.id = s.new_id;

-- Point each application in a later channel at that channel's copy.
UPDATE message_tags mt
SET tag_id = s.new_id
FROM posts p, tag_split s
WHERE p.id = mt.message_id AND s.old_id = mt.tag_id AND s.channel_id = p.channel_id
  AND s.new_id <> s.old_id;

-- Tags applied nowhere.
DELETE FROM tags WHERE owner_id IS NULL;

DROP TABLE tag_split;

ALTER TABLE tags
    ALTER COLUMN name_key SET NOT NULL,
    ALTER COLUMN owner_id SET NOT NULL,
    ALTER COLUMN scope SET NOT NULL,
    ADD CONSTRAINT tags_scope_check CHECK (scope IN ('private', 'channel')),
    ADD CONSTRAINT tags_scope_channel_check CHECK ((scope = 'channel') = (channel_id IS NOT NULL));

CREATE UNIQUE INDEX idx_tags_private_name ON tags (owner_id, name_key) WHERE scope = 'private';
CREATE UNIQUE INDEX idx_tags_channel_name ON tags (channel_id, name_key) WHERE scope = 'channel';
CREATE INDEX idx_tags_owner_id ON tags (owner_id);

-- Tags on channels.
CREATE TABLE channel_tags (
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    tag_id     UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_by UUID REFERENCES users(id) ON DELETE SET NULL,
    create_at  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (channel_id, tag_id)
);
CREATE INDEX idx_channel_tags_tag_id ON channel_tags (tag_id);

COMMIT;
