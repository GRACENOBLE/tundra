-- Rollback: add_username_to_users
-- Created at: 20251111133353
-- Remove username column and restore name column

-- Drop username index
DROP INDEX IF EXISTS idx_users_username;

-- Remove username column
ALTER TABLE users DROP COLUMN IF EXISTS username;

-- Restore name column
ALTER TABLE users ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT '';
