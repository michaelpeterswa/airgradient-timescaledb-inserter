-- Ownership is left to whichever role runs the migration rather than named
-- explicitly, so this works as the CloudNativePG application user, which owns
-- the database but is not a superuser.
CREATE SCHEMA IF NOT EXISTS sensors;
