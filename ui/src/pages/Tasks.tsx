import { useEffect, useState } from 'react';
import { api } from '../services/api';
import { SyncTask, Server } from '../types';

const Tasks = () => {
  const [tasks, setTasks] = useState<SyncTask[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [selectedServer, setSelectedServer] = useState<string>('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, [selectedServer]);

  const fetchData = async () => {
    try {
      const [serversRes, tasksRes] = await Promise.all([
        api.getServers(),
        api.getTasks(selectedServer || undefined),
      ]);
      setServers(serversRes.data);
      setTasks(tasksRes.data);
    } catch (error) {
      console.error('Failed to fetch tasks:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800';
      case 'failed':
        return 'bg-red-100 text-red-800';
      case 'processing':
        return 'bg-blue-100 text-blue-800';
      case 'pending':
        return 'bg-yellow-100 text-yellow-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'completed':
        return '✓ Completed';
      case 'failed':
        return '✗ Failed';
      case 'processing':
        return '⟳ Processing';
      case 'pending':
        return '⏳ Pending';
      default:
        return status;
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-500">Loading tasks...</div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Tasks</h1>
        <select
          value={selectedServer}
          onChange={(e) => setSelectedServer(e.target.value)}
          className="px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="">All Servers</option>
          {servers.map((server) => (
            <option key={server.id} value={server.id}>
              {server.name}
            </option>
          ))}
        </select>
      </div>

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Server</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Direction</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">File</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Status</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Progress</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Created</th>
            </tr>
          </thead>
          <tbody>
            {tasks.length === 0 ? (
              <tr>
                <td colSpan={6} className="p-8 text-center text-gray-500">
                  No tasks found.
                </td>
              </tr>
            ) : (
              tasks.map((task) => {
                const server = servers.find((s) => s.id === task.server_id);
                return (
                  <tr key={task.id} className="border-t hover:bg-gray-50">
                    <td className="p-3">{server?.name || task.server_id}</td>
                    <td className="p-3">
                      <span className={`px-2 py-1 rounded text-xs ${
                        task.direction === 'upload' ? 'bg-blue-100 text-blue-800' : 'bg-purple-100 text-purple-800'
                      }`}>
                        {task.direction === 'upload' ? '↑ Upload' : '↓ Download'}
                      </span>
                    </td>
                    <td className="p-3 font-mono text-sm">{task.file_name}</td>
                    <td className="p-3">
                      <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(task.status)}`}>
                        {getStatusText(task.status)}
                      </span>
                    </td>
                    <td className="p-3">
                      <div className="w-32 bg-gray-200 rounded-full h-2">
                        <div
                          className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${task.progress || 0}%` }}
                        />
                      </div>
                      <span className="text-xs text-gray-500 mt-1 block">
                        {Math.round(task.progress || 0)}%
                      </span>
                    </td>
                    <td className="p-3 text-sm text-gray-500">
                      {new Date(task.created_at).toLocaleString()}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default Tasks;