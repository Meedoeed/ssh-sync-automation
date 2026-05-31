ALTER TABLE sync_tasks DROP CONSTRAINT IF EXISTS sync_tasks_direction_check;
ALTER TABLE sync_tasks ADD CONSTRAINT sync_tasks_direction_check 
  CHECK (direction IN ('upload', 'download'));
ALTER TABLE sync_tasks ALTER COLUMN direction TYPE VARCHAR(10);