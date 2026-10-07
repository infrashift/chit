-- REFUSES rather than repairs. Restoring NOT NULL with machine actors present
-- would have to either delete them or invent a kratos_id and an email for each,
-- and a down-migration is the worst possible place to make either choice
-- silently. Remove them deliberately first, then run this.
DO $$
DECLARE machines BIGINT;
BEGIN
    SELECT count(*) INTO machines FROM users WHERE kratos_id IS NULL OR email IS NULL;
    IF machines > 0 THEN
        RAISE EXCEPTION
            'cannot restore NOT NULL: % user row(s) have no kratos_id or no email (machine actors). Delete them explicitly, then re-run.', machines;
    END IF;
END $$;

ALTER TABLE users ALTER COLUMN kratos_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
