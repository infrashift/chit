-- This FAILS once any direct or group channel exists, deliberately left so.
-- Their names are member IDs joined with "__" (a DM's is 74 characters) and
-- their display names list the members, so neither fits back into 64. Delete
-- those channels first if this migration must be reversed.
ALTER TABLE channels ALTER COLUMN name TYPE VARCHAR(64);
ALTER TABLE channels ALTER COLUMN display_name TYPE VARCHAR(64);
