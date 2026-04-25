import { useEffect, useState } from 'react';
import { api } from '../services/api';
import { WorkerStats, Server, SyncTask } from '../types';

const Dashboard = () => {
  const [stats, setStats] = useState<WorkerStats | null>(null);
  const [servers, setServers] = useState<Server[]>([]);
  const [tasks, setTasks] = useState<SyncTask[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [statsRes, serversRes, tasksRes] = await Promise.all([
          api.getWorkerStats(),
          api.getServers(),
          api.getTasks(),
        ]);
        setStats(statsRes.data);
        setServers(serversRes.data);
        setTasks(tasksRes.data);
      } catch (error) {
        console.error('Ошибка загрузки данных:', error);
      } finally {
        setLoading(false);
      }
    };
    
    fetchData();
    const interval = setInterval(fetchData, 10000); // обновление каждые 10 секунд
    return () => clearInterval(interval);
  }, []);

  if (loading) return <div className="text-center py-10">Загрузка...</div>;

  const onlineCount = stats?.workers.filter(w => w.state === 'running').length || 0;
  const totalTasks = tasks.length;
  const failedTasks = tasks.filter(t => t.status === 'failed').length;
  const pendingTasks = tasks.filter(t => t.status === 'pending' || t.status === 'processing').length;

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Панель управления</h1>
      
      {/* Карточки статистики */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-white rounded-lg shadow p-4">
          <div className="text-gray-500 text-sm">Всего серверов</div>
          <div className="text-2xl font-bold">{servers.length}</div>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <div className="text-gray-500 text-sm">Активных воркеров</div>
          <div className="text-2xl font-bold text-green-600">{onlineCount}</div>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <div className="text-gray-500 text-sm">Всего задач</div>
          <div className="text-2xl font-bold">{totalTasks}</div>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <div className="text-gray-500 text-sm">Ошибок</div>
          <div className="text-2xl font-bold text-red-600">{failedTasks}</div>
        </div>
      </div>

      {/* Дополнительная статистика */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
        <div className="bg-white rounded-lg shadow p-4">
          <div className="text-gray-500 text-sm mb-2">В ожидании</div>
          <div className="text-2xl font-bold text-yellow-600">{pendingTasks}</div>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <div className="text-gray-500 text-sm mb-2">Активных серверов</div>
          <div className="text-2xl font-bold text-blue-600">{servers.filter(s => s.is_active).length}</div>
        </div>
      </div>
      
      {/* Статус воркеров */}
      <div className="bg-white rounded-lg shadow mb-8">
        <div className="p-4 border-b font-semibold">Статус воркеров</div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="p-3 text-left">Сервер</th>
                <th className="p-3 text-left">Статус</th>
                <th className="p-3 text-left">Последняя синхронизация</th>
                <th className="p-3 text-left">Успешно / Ошибок</th>
              </tr>
            </thead>
            <tbody>
              {stats?.workers.length === 0 ? (
                <tr>
                  <td colSpan={4} className="p-8 text-center text-gray-500">
                    Нет активных воркеров. Добавьте сервер для начала синхронизации.
                  </td>
                </tr>
              ) : (
                stats?.workers.map(worker => (
                  <tr key={worker.server_id} className="border-t hover:bg-gray-50">
                    <td className="p-3 font-medium">{worker.server_name}</td>
                    <td className="p-3">
                      <span className={`px-2 py-1 rounded text-xs font-medium ${
                        worker.state === 'running' ? 'bg-green-100 text-green-800' :
                        worker.state === 'error' ? 'bg-red-100 text-red-800' :
                        'bg-gray-100 text-gray-800'
                      }`}>
                        {worker.state === 'running' ? '🟢 Работает' : 
                         worker.state === 'error' ? '🔴 Ошибка' : '⚫ Остановлен'}
                      </span>
                    </td>
                    <td className="p-3 text-sm">
                      {worker.last_sync ? new Date(worker.last_sync).toLocaleString() : '-'}
                    </td>
                    <td className="p-3">
                      <span className="text-green-600">✓ {worker.sync_count}</span>
                      {' / '}
                      <span className="text-red-600">✗ {worker.error_count}</span>
                      {worker.last_error && (
                        <div className="text-xs text-red-500 mt-1 truncate max-w-xs">
                          {worker.last_error}
                        </div>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Последние задачи */}
      <div className="bg-white rounded-lg shadow">
        <div className="p-4 border-b font-semibold">Последние задачи</div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="p-3 text-left">Файл</th>
                <th className="p-3 text-left">Направление</th>
                <th className="p-3 text-left">Статус</th>
                <th className="p-3 text-left">Прогресс</th>
                <th className="p-3 text-left">Время</th>
              </tr>
            </thead>
            <tbody>
              {tasks.length === 0 ? (
                <tr>
                  <td colSpan={5} className="p-8 text-center text-gray-500">
                    Нет задач. Загрузите файлы для начала синхронизации.
                  </td>
                </tr>
              ) : (
                tasks.slice(0, 5).map(task => {
                  const server = servers.find(s => s.id === task.server_id);
                  return (
                    <tr key={task.id} className="border-t hover:bg-gray-50">
                      <td className="p-3">
                        <div className="font-mono text-sm">{task.file_name}</div>
                        <div className="text-xs text-gray-500">{server?.name}</div>
                      </td>
                      <td className="p-3">
                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                          task.direction === 'upload' ? 'bg-blue-100 text-blue-800' : 'bg-purple-100 text-purple-800'
                        }`}>
                          {task.direction === 'upload' ? '↑ Загрузка' : '↓ Скачивание'}
                        </span>
                      </td>
                      <td className="p-3">
                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                          task.status === 'completed' ? 'bg-green-100 text-green-800' :
                          task.status === 'failed' ? 'bg-red-100 text-red-800' :
                          task.status === 'processing' ? 'bg-blue-100 text-blue-800' :
                          'bg-yellow-100 text-yellow-800'
                        }`}>
                          {task.status === 'completed' ? '✓ Завершена' :
                           task.status === 'failed' ? '✗ Ошибка' :
                           task.status === 'processing' ? '⟳ Выполняется' :
                           '⏳ Ожидает'}
                        </span>
                      </td>
                      <td className="p-3">
                        <div className="w-24 bg-gray-200 rounded-full h-2">
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
    </div>
  );
};

export default Dashboard;