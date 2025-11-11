-- Migration: latest_schemas
-- Created at: 20251111080517
-- Update users table to match GORM User model

-- Drop and recreate users table with correct schema
DROP TABLE IF EXISTS users CASCADE;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL
);

-- Create unique index on email
CREATE UNIQUE INDEX idx_users_email ON users(email);
