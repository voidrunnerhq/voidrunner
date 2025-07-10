-- Remove name field from users table
DROP INDEX IF EXISTS idx_users_name;
ALTER TABLE users DROP COLUMN IF EXISTS name;