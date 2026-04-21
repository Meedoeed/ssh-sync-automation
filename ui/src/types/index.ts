export interface Server {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  auth_type: 'password' | 'key';
  is_active: boolean;
  created_at: string;
  updated_at: string;
  last_seen?: string;
}

export interface WorkerStats {
  total_workers: number;
  workers: Worker[];
}

export interface Worker {
  server_id: string;
  server_name: string;
  state: 'running' | 'stopped' | 'error';
  last_sync: string;
  sync_count: number;
  error_count: number;
  last_error: string;
}

export interface SyncTask {
  id: string;
  server_id: string;
  direction: 'upload' | 'download';
  file_name: string;
  status: 'pending' | 'processing' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  file_size?: number;
  bytes_transferred?: number;
  error_message?: string;
  created_at: string;
  completed_at?: string;
}