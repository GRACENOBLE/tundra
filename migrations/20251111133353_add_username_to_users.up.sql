-- Migration: add_username_to_users
-- Created at: 20251111133353
-- Add username column to users table

-- First, drop the name column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS name;

-- Add username column
ALTER TABLE users ADD COLUMN username VARCHAR(255) NOT NULL DEFAULT '';

-- Add unique constraint on username
CREATE UNIQUE INDEX idx_users_username ON users(username);
