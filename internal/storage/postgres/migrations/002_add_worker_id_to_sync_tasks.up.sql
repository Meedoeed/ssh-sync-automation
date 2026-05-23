ALTER TABLE sync_tasks ADD COLUMN worker_id VARCHAR(255);

CREATE INDEX idx_sync_tasks_worker_id ON sync_tasks(worker_id);
