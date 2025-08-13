-- Create task_logs table for log storage with partitioning support
-- This migration creates a partitioned table structure required by the logging service

-- Create partitioned task_logs table (partitioned by date)
CREATE TABLE task_logs (
    id BIGSERIAL,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    execution_id UUID REFERENCES task_executions(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    stream VARCHAR(10) NOT NULL CHECK (stream IN ('stdout', 'stderr')),
    sequence_number BIGINT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create function to create daily partitions
CREATE OR REPLACE FUNCTION create_task_logs_partition(partition_date DATE)
RETURNS TEXT AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    -- Generate partition name in format task_logs_YYYY_MM_DD
    partition_name := 'task_logs_' || to_char(partition_date, 'YYYY_MM_DD');
    
    -- Set date bounds for the partition
    start_date := partition_date;
    end_date := partition_date + INTERVAL '1 day';
    
    -- Check if partition already exists
    IF EXISTS (
        SELECT 1 FROM pg_tables 
        WHERE tablename = partition_name 
        AND schemaname = 'public'
    ) THEN
        RETURN 'Partition ' || partition_name || ' already exists';
    END IF;
    
    -- Create the partition
    EXECUTE format(
        'CREATE TABLE %I PARTITION OF task_logs FOR VALUES FROM (%L) TO (%L)',
        partition_name,
        start_date,
        end_date
    );
    
    -- Create indexes on the partition
    EXECUTE format('CREATE INDEX %I ON %I (task_id, execution_id, sequence_number)', 
                   'idx_' || partition_name || '_task_execution', partition_name);
    EXECUTE format('CREATE INDEX %I ON %I (timestamp DESC)', 
                   'idx_' || partition_name || '_timestamp', partition_name);
    EXECUTE format('CREATE INDEX %I ON %I (task_id)', 
                   'idx_' || partition_name || '_task_id', partition_name);
    
    RETURN 'Created partition ' || partition_name || ' for date range [' || start_date || ', ' || end_date || ')';
END;
$$ LANGUAGE plpgsql;

-- Create function to get partition statistics
CREATE OR REPLACE FUNCTION get_task_logs_partition_stats()
RETURNS TABLE (
    partition_name TEXT,
    row_count BIGINT,
    size_bytes BIGINT,
    partition_date DATE
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        t.tablename::TEXT,
        COALESCE(s.n_tup_ins + s.n_tup_upd - s.n_tup_del, 0) AS row_count,
        COALESCE(pg_total_relation_size(('public.' || t.tablename)::regclass), 0) AS size_bytes,
        CASE 
            WHEN t.tablename ~ '^task_logs_\d{4}_\d{2}_\d{2}$' THEN
                to_date(substring(t.tablename from 11), 'YYYY_MM_DD')
            ELSE
                NULL
        END AS partition_date
    FROM pg_tables t
    LEFT JOIN pg_stat_user_tables s ON s.relname = t.tablename
    WHERE t.tablename LIKE 'task_logs_%'
    AND t.schemaname = 'public'
    AND t.tablename ~ '^task_logs_\d{4}_\d{2}_\d{2}$'
    ORDER BY partition_date DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;

-- Create function to cleanup old partitions
CREATE OR REPLACE FUNCTION cleanup_old_task_logs_partitions(retention_days INTEGER)
RETURNS TEXT AS $$
DECLARE
    partition_record RECORD;
    cutoff_date DATE;
    dropped_count INTEGER := 0;
    result_text TEXT := '';
BEGIN
    -- Calculate cutoff date
    cutoff_date := CURRENT_DATE - retention_days;
    
    -- Find and drop old partitions
    FOR partition_record IN 
        SELECT tablename,
               to_date(substring(tablename from 11), 'YYYY_MM_DD') as partition_date
        FROM pg_tables 
        WHERE tablename LIKE 'task_logs_%'
        AND schemaname = 'public'
        AND tablename ~ '^task_logs_\d{4}_\d{2}_\d{2}$'
        AND to_date(substring(tablename from 11), 'YYYY_MM_DD') < cutoff_date
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I CASCADE', partition_record.tablename);
        dropped_count := dropped_count + 1;
        
        IF result_text != '' THEN
            result_text := result_text || ', ';
        END IF;
        result_text := result_text || partition_record.tablename;
    END LOOP;
    
    IF dropped_count = 0 THEN
        RETURN 'No partitions found older than ' || retention_days || ' days (cutoff: ' || cutoff_date || ')';
    ELSE
        RETURN 'Dropped ' || dropped_count || ' partition(s): ' || result_text;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Create initial partition for current date (minimum required)
-- Additional partitions will be created as needed by the application
SELECT create_task_logs_partition(CURRENT_DATE);

-- Validation: Ensure the task_logs table exists and is properly configured
DO $$
BEGIN
    -- Check that the task_logs table exists
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'task_logs' AND table_schema = 'public') THEN
        RAISE EXCEPTION 'task_logs table was not created successfully';
    END IF;

    -- Check that the create_task_logs_partition function exists
    IF NOT EXISTS (SELECT 1 FROM information_schema.routines WHERE routine_name = 'create_task_logs_partition' AND routine_schema = 'public') THEN
        RAISE EXCEPTION 'create_task_logs_partition function was not created successfully';
    END IF;

    -- Check that at least one partition was created
    IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE tablename LIKE 'task_logs_%' AND schemaname = 'public') THEN
        RAISE EXCEPTION 'No task_logs partitions were created - initial partition creation failed';
    END IF;

    RAISE NOTICE 'Migration 005_task_logs_table validation passed: table, function, and initial partition created successfully';
END $$;

-- Add comments
COMMENT ON TABLE task_logs IS 'Stores real-time container execution logs - partitioned by date for performance';
COMMENT ON COLUMN task_logs.sequence_number IS 'Monotonically increasing sequence number within each task execution to ensure log ordering';
COMMENT ON COLUMN task_logs.stream IS 'Log stream type: stdout for standard output, stderr for error output';
COMMENT ON COLUMN task_logs.content IS 'Raw log line content from container execution';
COMMENT ON FUNCTION create_task_logs_partition(DATE) IS 'Creates a new daily partition for task logs';
COMMENT ON FUNCTION get_task_logs_partition_stats() IS 'Returns statistics about all task log partitions';
COMMENT ON FUNCTION cleanup_old_task_logs_partitions(INTEGER) IS 'Removes partitions older than specified number of days';