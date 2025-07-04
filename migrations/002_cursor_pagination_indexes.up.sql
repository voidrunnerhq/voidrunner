-- Add indexes optimized for cursor-based pagination and performance
-- These indexes support efficient cursor-based pagination using (sort_field, id) composite keys

-- Cursor-based pagination indexes for tasks
-- Primary cursor index: user_id + created_at + id (most common query pattern)
CREATE INDEX idx_tasks_user_created_cursor ON tasks(user_id, created_at DESC, id);

-- Status-based cursor pagination
CREATE INDEX idx_tasks_status_created_cursor ON tasks(status, created_at DESC, id);

-- Priority-based cursor pagination (for priority-sorted lists)
CREATE INDEX idx_tasks_priority_created_cursor ON tasks(priority DESC, created_at DESC, id);

-- User + status combination (common filter pattern)
CREATE INDEX idx_tasks_user_status_created ON tasks(user_id, status, created_at DESC, id);

-- Cursor-based pagination indexes for task executions
-- Task executions by task_id (most common query)
CREATE INDEX idx_executions_task_created_cursor ON task_executions(task_id, created_at DESC, id);

-- Status-based execution queries
CREATE INDEX idx_executions_status_created_cursor ON task_executions(status, created_at DESC, id);

-- Performance optimization indexes
-- Composite index for common task queries (replaces single-column indexes)
CREATE INDEX idx_tasks_user_status_priority ON tasks(user_id, status, priority DESC);

-- Execution performance indexes
CREATE INDEX idx_executions_status_started ON task_executions(status, started_at DESC) WHERE started_at IS NOT NULL;

-- Covering index for task list queries (includes commonly selected columns)
CREATE INDEX idx_tasks_list_covering ON tasks(user_id, created_at DESC) 
INCLUDE (name, status, priority, script_type);

-- Covering index for execution queries
CREATE INDEX idx_executions_list_covering ON task_executions(task_id, created_at DESC) 
INCLUDE (status, return_code, execution_time_ms);

-- Partial indexes for active/running tasks (common administrative queries)
CREATE INDEX idx_tasks_active_status ON tasks(status, created_at DESC) 
WHERE status IN ('pending', 'running');

CREATE INDEX idx_executions_active_status ON task_executions(status, created_at DESC) 
WHERE status IN ('pending', 'running');

-- JSONB optimization for metadata searches
CREATE INDEX idx_tasks_metadata_jsonb_path_ops ON tasks USING GIN(metadata jsonb_path_ops);