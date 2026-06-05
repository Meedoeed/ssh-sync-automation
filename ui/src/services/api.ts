import { createPromiseClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { BackendService } from "../gen/proto/service_connect";
import type {
  ListServersResponse,
  GetServerByIdResponse,
  CreateServerResponse,
  UpdateServerResponse,
  DeleteServerResponse,
  GetWorkerStatsResponse,
  ListTasksResponse,
  GetTaskByIdResponse,
  HealthCheckResponse,
  ServerProto,
  TaskProto,
  WorkerStatProto,
} from "../gen/proto/service_pb";

const RPC_URL = process.env.REACT_APP_RPC_URL || "";

const transport = createConnectTransport({
    baseUrl: RPC_URL,
});

const client = createPromiseClient(BackendService, transport);

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
  password?: string;
  private_key?: string;
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

export interface WorkerStats {
  total_workers: number;
  workers: Worker[];
}

export interface SyncTask {
  id: string;
  server_id: string;
  direction: 'upload' | 'download';
  file_name: string;
  status: string;
  progress: number;
  file_size?: number;
  bytes_transferred?: number;
  error_message?: string;
  created_at: string;
  completed_at?: string;
}

function protoToServer(proto: ServerProto): Server {
  return {
    id: proto.id,
    name: proto.name,
    host: proto.host,
    port: proto.port,
    username: proto.username,
    auth_type: proto.authType as 'password' | 'key',
    is_active: proto.isActive,
    created_at: proto.createdAt,
    updated_at: proto.updatedAt,
    last_seen: proto.lastSeen,
    password: proto.password,
    private_key: proto.privateKey,
  };
}

function protoToTask(proto: TaskProto): SyncTask {
  return {
    id: proto.id,
    server_id: proto.serverId,
    direction: proto.direction as 'upload' | 'download',
    file_name: proto.fileName,
    status: proto.status,
    progress: proto.progress,
    file_size: Number(proto.fileSize),
    bytes_transferred: Number(proto.bytesTransferred),
    error_message: proto.errorMessage,
    created_at: proto.createdAt,
    completed_at: proto.completedAt,
  };
}

export const api = {
  getServers: async (activeOnly?: boolean): Promise<{ data: Server[] }> => {
    const response = await client.listServers({ activeOnly: activeOnly || false }) as ListServersResponse;
    return { data: response.servers.map(protoToServer) };
  },

  getServer: async (id: string): Promise<{ data: Server }> => {
    const response = await client.getServerById({ serverId: id }) as GetServerByIdResponse;
    if (!response.server) {
      throw new Error(`Server ${id} not found`);
    }
    return { data: protoToServer(response.server) };
  },

  createServer: async (data: {
    name: string;
    host: string;
    port: number;
    username: string;
    auth_type: 'password' | 'key';
    is_active?: boolean;
    password?: string;
    private_key?: string;
  }): Promise<{ data: Server }> => {
    const response = await client.createServer({
      name: data.name,
      host: data.host,
      port: data.port,
      username: data.username,
      authType: data.auth_type,
      isActive: data.is_active ?? true,
      password: data.password,
      privateKey: data.private_key,
    }) as CreateServerResponse;
    if (!response.server) {
      throw new Error("Failed to create server: no response");
    }
    return { data: protoToServer(response.server) };
  },

  updateServer: async (id: string, data: {
    name?: string;
    host?: string;
    port?: number;
    username?: string;
    auth_type?: 'password' | 'key';
    is_active?: boolean;
    password?: string;
    private_key?: string;
  }): Promise<{ data: Server }> => {
    const response = await client.updateServer({
      serverId: id,
      name: data.name,
      host: data.host,
      port: data.port,
      username: data.username,
      authType: data.auth_type,
      isActive: data.is_active,
      password: data.password,
      privateKey: data.private_key,
    }) as UpdateServerResponse;
    if (!response.server) {
      throw new Error(`Failed to update server ${id}`);
    }
    return { data: protoToServer(response.server) };
  },

  deleteServer: async (id: string): Promise<void> => {
    const response = await client.deleteServer({ serverId: id }) as DeleteServerResponse;
    if (!response.success) {
      throw new Error(`Failed to delete server ${id}`);
    }
  },

  getWorkerStats: async (): Promise<{ data: WorkerStats }> => {
    const response = await client.getWorkerStats({}) as GetWorkerStatsResponse;
    return {
      data: {
        total_workers: response.totalWorkers,
        workers: response.workers.map((w: WorkerStatProto) => ({
          server_id: w.serverId,
          server_name: w.serverName,
          state: w.state as 'running' | 'stopped' | 'error',
          last_sync: w.lastSync,
          sync_count: Number(w.syncCount),
          error_count: Number(w.errorCount),
          last_error: w.lastError,
        })),
      },
    };
  },

  getTasks: async (serverId?: string): Promise<{ data: SyncTask[] }> => {
    const response = await client.listTasks({
      serverId: serverId,
      limit: 100,
    }) as ListTasksResponse;
    return { data: response.tasks.map(protoToTask) };
  },

  getTask: async (id: string): Promise<{ data: SyncTask }> => {
    const response = await client.getTaskById({ taskId: id }) as GetTaskByIdResponse;
    if (!response.task) {
      throw new Error(`Task ${id} not found`);
    }
    return { data: protoToTask(response.task) };
  },

  healthCheck: async (): Promise<{ status: string }> => {
    const response = await client.healthCheck({}) as HealthCheckResponse;
    return { status: response.status };
  },
};