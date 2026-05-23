DROP INDEX IF EXISTS idx_sync_tasks_worker_id;
ALTER TABLE sync_tasks DROP COLUMN IF EXISTS worker_id;