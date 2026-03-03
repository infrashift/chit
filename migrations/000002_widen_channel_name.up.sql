-- DM channel names are "uuid1__uuid2" (74 chars) and group channel names can
-- be up to 8 UUIDs joined by "__" (36*8 + 2*7 = 302 chars).  Widen both name
-- and display_name to accommodate.

ALTER TABLE channels ALTER COLUMN name TYPE VARCHAR(512);
ALTER TABLE channels ALTER COLUMN display_name TYPE VARCHAR(256);
