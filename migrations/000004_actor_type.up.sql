-- Actors: humans, AI agents, and bots are all rows in users, distinguished
-- only by actor_type. This realizes the "humans, bots, and agents are equal"
-- design: same tables, same permissions machinery, explicit type for audit
-- and policy purposes.
ALTER TABLE users
    ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'user'
    CHECK (actor_type IN ('user', 'agent', 'bot'));
