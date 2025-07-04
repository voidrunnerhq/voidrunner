-- Drop triggers
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (due to foreign key constraints)
DROP TABLE IF EXISTS task_executions;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS users;

-- Drop extension (only if no other tables use it)
-- DROP EXTENSION IF EXISTS "pgcrypto";