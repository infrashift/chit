-- Drops the attachment tables. LOSSY: the rows go, and the objects they
-- name stay in the bucket, where nothing refers to them any more.

BEGIN;

DROP TABLE post_files;
DROP TABLE files;

COMMIT;
