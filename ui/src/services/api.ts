import axios from 'axios';
import { Server, WorkerStats, SyncTask } from '../types';

const API_BASE = process.env.REACT_APP_API_URL || 'http://localhost:8081/api/v1';

const apiClient = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('API Error:', error.response?.data || error.message);
    return Promise.reject(error);
  }
);

export const api = {
  getServers: () => apiClient.get<Server[]>('/servers'),
  getServer: (id: string) => apiClient.get<Server>(`/servers/${id}`),
  createServer: (data: Partial<Server>) => apiClient.post<Server>('/servers', data),
  updateServer: (id: string, data: Partial<Server>) => apiClient.put<Server>(`/servers/${id}`, data),
  deleteServer: (id: string) => apiClient.delete(`/servers/${id}`),
  
  getWorkerStats: () => apiClient.get<WorkerStats>('/workers/stats'),
  
  getTasks: (serverId?: string) => apiClient.get<SyncTask[]>('/tasks', { params: { server_id: serverId } }),
};