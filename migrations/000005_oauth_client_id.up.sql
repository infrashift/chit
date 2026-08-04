-- Map an OAuth2 client (Ory Hydra, client_credentials grant) to a Chit user.
--
-- Hydra's client_credentials tokens carry the generated client_id as `sub`,
-- and Hydra does not surface client metadata through token introspection for
-- that grant. Oathkeeper therefore forwards the client_id as X-Client-Id and
-- chitd resolves it to a user here.
--
-- NULL for every human user; unique so one client maps to at most one user.
-- Postgres permits multiple NULLs under a UNIQUE constraint, which is exactly
-- the semantics we want — write NULL, never the empty string.
ALTER TABLE users
    ADD COLUMN oauth_client_id TEXT UNIQUE;

COMMENT ON COLUMN users.oauth_client_id IS
    'Ory Hydra OAuth2 client_id for machine actors (agents/bots); NULL for humans.';
