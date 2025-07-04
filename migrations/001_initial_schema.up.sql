-- Enable UUID extension for gen_random_uuid() function
-- Note: gen_random_uuid() is built-in to PostgreSQL 13+, but we ensure compatibility with older versions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index for email lookups
CREATE INDEX idx_users_email ON users(email);

-- Create tasks table
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    script_content TEXT NOT NULL,
    script_type VARCHAR(50) NOT NULL DEFAULT 'python',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 0,
    timeout_seconds INTEGER NOT NULL DEFAULT 10,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_script_type CHECK (script_type IN ('python', 'javascript', 'bash', 'go')),
    CONSTRAINT chk_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout', 'cancelled')),
    CONSTRAINT chk_priority CHECK (priority >= 0 AND priority <= 10),
    CONSTRAINT chk_timeout CHECK (timeout_seconds > 0 AND timeout_seconds <= 3600)
);

-- Create indexes for tasks table
CREATE INDEX idx_tasks_user_status ON tasks(user_id, status);
CREATE INDEX idx_tasks_created_at ON tasks(created_at);
CREATE INDEX idx_tasks_priority_status ON tasks(priority DESC, status);
CREATE INDEX idx_tasks_metadata_gin ON tasks USING GIN(metadata);

-- Create task_executions table
CREATE TABLE task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    return_code INTEGER,
    stdout TEXT,
    stderr TEXT,
    execution_time_ms INTEGER,
    memory_usage_bytes BIGINT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_execution_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout', 'cancelled')),
    CONSTRAINT chk_return_code CHECK (return_code >= 0 AND return_code <= 255),
    CONSTRAINT chk_execution_time CHECK (execution_time_ms >= 0),
    CONSTRAINT chk_memory_usage CHECK (memory_usage_bytes >= 0)
);

-- Create indexes for task_executions table
CREATE INDEX idx_executions_task_created ON task_executions(task_id, created_at DESC);
CREATE INDEX idx_executions_status_created ON task_executions(status, created_at DESC);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();