-- Add name field to users table
ALTER TABLE users ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT '';

-- Create index for name searches (optional, but useful for future features)
CREATE INDEX idx_users_name ON users(name);