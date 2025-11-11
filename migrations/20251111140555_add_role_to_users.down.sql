-- Rollback: add_role_to_users
-- Created at: 20251111140555
-- Remove role column from users table

-- Drop the index
DROP INDEX IF EXISTS idx_users_role;

-- Remove role column
ALTER TABLE users DROP COLUMN IF EXISTS role;
