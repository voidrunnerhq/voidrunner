-- Remove enhanced indexes for cursor pagination with different sort fields

-- Priority-based cursor pagination indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_user_priority_created_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_user_priority_asc_created_id;

-- Updated_at-based cursor pagination indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_user_updated_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_user_updated_asc_id;

-- Name-based cursor pagination indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_user_name_created_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_user_name_asc_created_id;

-- Global indexes for status filtering with different sort fields
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_status_priority_created_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_status_updated_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_status_name_created_id;

-- Global indexes for all tasks with different sort fields
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_priority_created_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_updated_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_name_created_id;

-- Complex composite indexes
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_priority_status_user_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_tasks_name_text_pattern;