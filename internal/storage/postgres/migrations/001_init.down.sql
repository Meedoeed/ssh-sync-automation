DROP TRIGGER IF EXISTS update_sync_tasks_updated_at ON sync_tasks;
DROP TRIGGER IF EXISTS update_servers_updated_at ON servers;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS sync_tasks;
DROP TABLE IF EXISTS server_status;
DROP TABLE IF EXISTS servers;