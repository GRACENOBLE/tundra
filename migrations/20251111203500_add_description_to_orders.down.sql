-- Remove description column from orders table
ALTER TABLE orders DROP COLUMN IF EXISTS description;
