-- Let a MACHINE ACTOR exist.
--
-- 000005 added users.oauth_client_id so an Ory Hydra client_credentials caller
-- could resolve to a user, and chitd has resolved X-Client-Id through it ever
-- since. Nothing could ever be written for it to find: kratos_id and email were
-- both UNIQUE NOT NULL, so every row required a person's identity and a
-- mailbox. A machine has neither, and giving it a synthetic pair to satisfy the
-- schema is how a machine ends up indistinguishable from a person in the one
-- table that decides who somebody is.
--
-- UNIQUE stays on both. Postgres permits many NULLs under a UNIQUE constraint
-- and exactly one '', which is the same reason 000005 wrote NULL rather than
-- the empty string, and why the store now NULLIFs these on write and COALESCEs
-- them on read.
--
-- A person is still required to have both: model.User.IsValid enforces
-- kratos_id => email, and refuses a user identified by neither column. The
-- database no longer states that rule because it can no longer state it for
-- every row.
ALTER TABLE users ALTER COLUMN kratos_id DROP NOT NULL;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

COMMENT ON COLUMN users.kratos_id IS
    'Ory Kratos identity for people; NULL for machine actors, which are identified by oauth_client_id.';
COMMENT ON COLUMN users.email IS
    'Email for people, from their Kratos identity; NULL for machine actors.';
