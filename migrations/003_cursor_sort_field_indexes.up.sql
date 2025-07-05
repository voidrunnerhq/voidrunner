-- Enhanced indexes for cursor pagination with different sort fields
-- These indexes support efficient cursor-based pagination for all sort fields

-- Existing cursor pagination indexes from 002_cursor_pagination_indexes.up.sql:
-- Index for created_at sorting (user-specific)
-- Index for status filtering with created_at sorting  
-- Index for user tasks with priority
-- etc.

-- Additional indexes for priority-based cursor pagination
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_priority_created_id 
ON tasks(user_id, priority DESC, created_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_priority_asc_created_id 
ON tasks(user_id, priority ASC, created_at ASC, id ASC);

-- Additional indexes for updated_at-based cursor pagination  
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_updated_id
ON tasks(user_id, updated_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_updated_asc_id
ON tasks(user_id, updated_at ASC, id ASC);

-- Additional indexes for name-based cursor pagination
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_name_created_id
ON tasks(user_id, name DESC, created_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_name_asc_created_id
ON tasks(user_id, name ASC, created_at ASC, id ASC);

-- Global indexes for status filtering with different sort fields
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_status_priority_created_id
ON tasks(status, priority DESC, created_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_status_updated_id
ON tasks(status, updated_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_status_name_created_id
ON tasks(status, name DESC, created_at DESC, id DESC);

-- Global indexes for all tasks with different sort fields (for ListCursor)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_priority_created_id
ON tasks(priority DESC, created_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_updated_id
ON tasks(updated_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_name_created_id
ON tasks(name DESC, created_at DESC, id DESC);

-- Composite index for complex priority-based queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_priority_status_user_created
ON tasks(priority DESC, status, user_id, created_at DESC)
WHERE deleted_at IS NULL;

-- Index for name sorting with text pattern matching (if needed for search)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_name_text_pattern
ON tasks(name text_pattern_ops)
WHERE deleted_at IS NULL;