CREATE TABLE IF NOT EXISTS servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL DEFAULT 22,
    username VARCHAR(255) NOT NULL,
    auth_type VARCHAR(20) NOT NULL CHECK (auth_type IN ('password', 'key')),
    password TEXT,
    private_key TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_servers_name ON servers(name);
CREATE INDEX idx_servers_active ON servers(is_active);

CREATE TABLE IF NOT EXISTS server_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL CHECK (status IN ('online', 'offline', 'syncing', 'error')),
    last_checked TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_server_status_server_id ON server_status(server_id);
CREATE INDEX idx_server_status_last_checked ON server_status(last_checked DESC);
CREATE INDEX idx_server_status_status ON server_status(status);

CREATE TABLE IF NOT EXISTS sync_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('upload', 'download')),
    file_name VARCHAR(512) NOT NULL,
    remote_path TEXT NOT NULL,
    local_path TEXT NOT NULL,
    file_size BIGINT DEFAULT 0,
    bytes_transferred BIGINT DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    attempt_count INT DEFAULT 0,
    max_attempts INT DEFAULT 5,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_sync_tasks_server_id ON sync_tasks(server_id);
CREATE INDEX idx_sync_tasks_status ON sync_tasks(status);
CREATE INDEX idx_sync_tasks_created_at ON sync_tasks(created_at DESC);
CREATE INDEX idx_sync_tasks_server_status ON sync_tasks(server_id, status);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_servers_updated_at
    BEFORE UPDATE ON servers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sync_tasks_updated_at
    BEFORE UPDATE ON sync_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();