-- Add description column to orders table
ALTER TABLE orders ADD COLUMN IF NOT EXISTS description TEXT;
