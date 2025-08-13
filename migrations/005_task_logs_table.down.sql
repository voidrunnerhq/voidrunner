-- Rollback migration for task_logs table
-- This script removes all log-related tables, functions, and indexes

-- Drop all task_logs partitions first
DO $$
DECLARE
    partition_record RECORD;
BEGIN
    -- Drop all task_logs partitions
    FOR partition_record IN 
        SELECT schemaname, tablename
        FROM pg_tables 
        WHERE tablename LIKE 'task_logs_%'
        AND schemaname = 'public'
        AND tablename ~ '^task_logs_\d{4}_\d{2}_\d{2}$'
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.%I CASCADE', 
                      partition_record.schemaname, 
                      partition_record.tablename);
    END LOOP;
END $$;

-- Drop the main partitioned table
-- This will cascade and remove any remaining indexes and constraints
DROP TABLE IF EXISTS task_logs CASCADE;

-- Drop all log-related functions
DROP FUNCTION IF EXISTS create_task_logs_partition(DATE);
DROP FUNCTION IF EXISTS cleanup_old_task_logs_partitions(INTEGER);
DROP FUNCTION IF EXISTS get_task_logs_partition_stats();
DROP FUNCTION IF EXISTS update_task_logs_updated_at();

-- Note: Sequences are automatically dropped when the table is dropped
-- Note: Indexes are automatically dropped when the table is dropped
-- Note: Constraints and triggers are automatically dropped when the table is dropped

-- The GRANT statements don't need explicit revocation as they're tied to the dropped objects