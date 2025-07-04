-- Drop all indexes created in the cursor pagination migration

-- Drop cursor-based pagination indexes for tasks
DROP INDEX IF EXISTS idx_tasks_user_created_cursor;
DROP INDEX IF EXISTS idx_tasks_status_created_cursor;
DROP INDEX IF EXISTS idx_tasks_priority_created_cursor;
DROP INDEX IF EXISTS idx_tasks_user_status_created;

-- Drop cursor-based pagination indexes for task executions
DROP INDEX IF EXISTS idx_executions_task_created_cursor;
DROP INDEX IF EXISTS idx_executions_status_created_cursor;

-- Drop performance optimization indexes
DROP INDEX IF EXISTS idx_tasks_user_status_priority;
DROP INDEX IF EXISTS idx_executions_status_started;

-- Drop covering indexes
DROP INDEX IF EXISTS idx_tasks_list_covering;
DROP INDEX IF EXISTS idx_executions_list_covering;

-- Drop partial indexes
DROP INDEX IF EXISTS idx_tasks_active_status;
DROP INDEX IF EXISTS idx_executions_active_status;

-- Drop JSONB optimization
DROP INDEX IF EXISTS idx_tasks_metadata_jsonb_path_ops;