-- Attachments (P-CHIT-5, HALO PLAN.md 3.6).
--
-- A file is an image that one person uploaded to one channel. The bytes live
-- in the S3 bucket (SeaweedFS "chit-files" in the deploy manifests) under
-- s3_key, "<channel_id>/<file_id>" of the upload that first stored them; the
-- row holds what Chit checked and what a reader needs.
--
-- DEDUPLICATION is per channel only: an upload whose re-encoded bytes have
-- the SHA-256 of a live file in the same channel gets its own row, which
-- points at that file's object (its s3_key). Two channels never share an
-- object, so deleting or leaving one channel cannot expose or remove what
-- the other holds.
--
-- uploader_actor_id is the person the upload is authorized as (the
-- principal); via_actor_id is the allowlisted client that uploaded on their
-- behalf (X-On-Behalf-Of), or NULL. Quotas and "attach only your own file"
-- count the uploader.
--
-- create_at and delete_at are milliseconds, as everywhere in Chit; a live
-- file has delete_at 0.

BEGIN;

CREATE TABLE files (
    id                UUID PRIMARY KEY,
    channel_id        UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    uploader_actor_id UUID NOT NULL REFERENCES users(id),
    via_actor_id      UUID REFERENCES users(id),
    name              VARCHAR(255) NOT NULL,
    sha256            CHAR(64) NOT NULL,
    content_type      VARCHAR(64) NOT NULL,
    size_bytes        BIGINT NOT NULL CHECK (size_bytes > 0),
    width             INTEGER NOT NULL CHECK (width > 0),
    height            INTEGER NOT NULL CHECK (height > 0),
    alt_text          VARCHAR(1000) NOT NULL DEFAULT '',
    s3_key            VARCHAR(80) NOT NULL,
    create_at         BIGINT NOT NULL,
    delete_at         BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT files_content_type_check CHECK (content_type IN ('image/png', 'image/jpeg'))
);

-- Deduplication: a live file with these bytes in this channel.
CREATE INDEX idx_files_channel_sha256 ON files (channel_id, sha256) WHERE delete_at = 0;
-- The quota: live bytes per uploader.
CREATE INDEX idx_files_uploader ON files (uploader_actor_id) WHERE delete_at = 0;
-- The sweep: whether any live row still uses an object.
CREATE INDEX idx_files_s3_key ON files (s3_key);
CREATE INDEX idx_files_delete_at ON files (delete_at) WHERE delete_at > 0;

-- A file is attached to at most one post (file_id is unique), in the
-- post's order (position).
CREATE TABLE post_files (
    post_id  UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (post_id, file_id),
    CONSTRAINT post_files_file_unique UNIQUE (file_id)
);

COMMIT;
