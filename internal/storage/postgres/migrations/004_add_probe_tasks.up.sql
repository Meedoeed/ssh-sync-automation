CREATE TABLE IF NOT EXISTS probe_tasks (
    id UUID PRIMARY KEY,
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    task_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    worker_id VARCHAR(255),
    files_found TEXT[] DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_probe_tasks_status ON probe_tasks(status);
CREATE INDEX idx_probe_tasks_server_id ON probe_tasks(server_id);