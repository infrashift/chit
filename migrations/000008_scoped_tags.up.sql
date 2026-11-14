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
-- Old names that share a key in one channel ("Bug" and "bug") merge into
-- one Tag, named as the one with the lower id.
--
-- name_key is lower(normalize(name, NFKC)): NFKC, then simple lowercasing,
-- the same rule as the server's model.TagNameKey. TestTagNameKey_AgreesWithPostgres
-- holds the two together. ß and ss stay distinct names.

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

-- Each (old tag, channel it is used in), with its new key.
CREATE TEMP TABLE tag_use AS
SELECT DISTINCT mt.tag_id AS old_id, p.channel_id, lower(normalize(t.name, NFKC)) AS name_key
FROM message_tags mt
JOIN posts p ON p.id = mt.message_id
JOIN tags t ON t.id = mt.tag_id;

-- One new tag per (channel, key). The old names were unique only as
-- written, so "Bug" and "bug" could both be used in one channel; they share
-- a key now and merge into one tag, named as the oldest by id ("keeper").
-- The keeper keeps its id in the first channel (by id) it is kept in.
CREATE TEMP TABLE tag_target AS
SELECT channel_id, name_key, keeper,
       CASE WHEN ROW_NUMBER() OVER (PARTITION BY keeper ORDER BY channel_id) = 1
            THEN keeper ELSE gen_random_uuid() END AS new_id
FROM (
    SELECT channel_id, name_key, MIN(old_id::text)::uuid AS keeper
    FROM tag_use
    GROUP BY channel_id, name_key
) groups;

CREATE TEMP TABLE tag_split AS
SELECT u.old_id, u.channel_id, g.new_id
FROM tag_use u JOIN tag_target g ON g.channel_id = u.channel_id AND g.name_key = u.name_key;

-- The new rows: copies for later channels.
INSERT INTO tags (id, name)
SELECT g.new_id, t.name
FROM tag_target g JOIN tags t ON t.id = g.keeper
WHERE g.new_id <> g.keeper;

UPDATE tags t
SET owner_id   = '00000000-0000-7000-8000-000000000001',
    scope      = 'channel',
    channel_id = g.channel_id,
    name_key   = g.name_key,
    create_at  = (extract(epoch FROM now()) * 1000)::bigint,
    update_at  = (extract(epoch FROM now()) * 1000)::bigint
FROM tag_target g
WHERE t.id = g.new_id;

-- Point every application at its channel's tag; a post that carried two
-- merged names keeps one application.
CREATE TEMP TABLE mt_new AS
SELECT DISTINCT mt.message_id, s.new_id AS tag_id
FROM message_tags mt
JOIN posts p ON p.id = mt.message_id
JOIN tag_split s ON s.old_id = mt.tag_id AND s.channel_id = p.channel_id;
DELETE FROM message_tags;
INSERT INTO message_tags (message_id, tag_id) SELECT message_id, tag_id FROM mt_new;

-- Tags applied nowhere, and merged names.
DELETE FROM tags WHERE owner_id IS NULL;

DROP TABLE mt_new;
DROP TABLE tag_split;
DROP TABLE tag_target;
DROP TABLE tag_use;

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
