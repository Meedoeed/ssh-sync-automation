CREATE TABLE IF NOT EXISTS scheduler_leader (
    id VARCHAR(50) PRIMARY KEY DEFAULT 'default',
    leader_id VARCHAR(255) NOT NULL,
    last_heartbeat TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_scheduler_leader_heartbeat ON scheduler_leader(last_heartbeat);