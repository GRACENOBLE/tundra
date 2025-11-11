-- Migration: add_role_to_users
-- Created at: 20251111140555
-- Add role column to users table

-- Add role column with default value 'user'
ALTER TABLE users ADD COLUMN role VARCHAR(50) NOT NULL DEFAULT 'user';

-- Create index on role for faster queries
CREATE INDEX idx_users_role ON users(role);

-- Optional: Update any existing users to have 'user' role if needed
-- UPDATE users SET role = 'user' WHERE role IS NULL;
