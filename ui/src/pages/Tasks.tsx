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

  const getStatusStyle = (status: string) => {
    switch (status) {
      case 'completed': return 'bg-emerald-50 text-emerald-600 border-emerald-100';
      case 'failed': return 'bg-rose-50 text-rose-600 border-rose-100';
      case 'processing': return 'bg-blue-50 text-blue-600 border-blue-100 animate-pulse';
      case 'pending': return 'bg-amber-50 text-amber-600 border-amber-100';
      default: return 'bg-slate-50 text-slate-500 border-slate-100';
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-96">
        <div className="w-8 h-8 border-2 border-slate-200 border-t-slate-900 rounded-full animate-spin"></div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto space-y-10">
      {/* Header */}
      <div className="flex justify-between items-end">
        <div className="space-y-1">
          <h1 className="text-4xl font-light tracking-tight text-slate-900">Задачи</h1>
          <p className="text-slate-500 text-sm uppercase tracking-widest font-semibold">Мониторинг синхронизации файлов</p>
        </div>
        
        <div className="relative group">
          <select
            value={selectedServer}
            onChange={(e) => setSelectedServer(e.target.value)}
            className="appearance-none bg-white border border-slate-200 text-slate-700 py-3 pl-6 pr-12 rounded-2xl text-sm font-bold focus:outline-none focus:ring-2 focus:ring-slate-900 transition-all cursor-pointer shadow-sm"
          >
            <option value="">Все сервера</option>
            {servers.map((server) => (
              <option key={server.id} value={server.id}>{server.name}</option>
            ))}
          </select>
          <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-slate-400">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" /></svg>
          </div>
        </div>
      </div>

      {/* Table Container */}
      <div className="bg-white rounded-[32px] border border-slate-100 shadow-sm overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead>
              <tr className="bg-slate-50/50">
                <th className="px-8 py-5 text-[11px] font-bold text-slate-400 uppercase tracking-widest">Сервер</th>
                <th className="px-6 py-5 text-[11px] font-bold text-slate-400 uppercase tracking-widest">Тип</th>
                <th className="px-6 py-5 text-[11px] font-bold text-slate-400 uppercase tracking-widest">Файл</th>
                <th className="px-6 py-5 text-[11px] font-bold text-slate-400 uppercase tracking-widest">Статус</th>
                <th className="px-6 py-5 text-[11px] font-bold text-slate-400 uppercase tracking-widest">Прогресс</th>
                <th className="px-8 py-5 text-right text-[11px] font-bold text-slate-400 uppercase tracking-widest">Дата</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-50">
              {tasks.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-8 py-20 text-center text-slate-400 font-medium">Задачи не найдены</td>
                </tr>
              ) : (
                tasks.map((task) => {
                  const server = servers.find((s) => s.id === task.server_id);
                  return (
                    <tr key={task.id} className="hover:bg-slate-50/30 transition-colors">
                      <td className="px-8 py-6">
                        <span className="text-sm font-semibold text-slate-800">{server?.name || task.server_id}</span>
                      </td>
                      <td className="px-6 py-6">
                        <span className={`px-2.5 py-1 rounded-lg text-[10px] font-bold uppercase tracking-wider ${
                          task.direction === 'upload' ? 'bg-blue-50 text-blue-600' : 'bg-indigo-50 text-indigo-600'
                        }`}>
                          {task.direction === 'upload' ? '↑ Upload' : '↓ Download'}
                        </span>
                      </td>
                      <td className="px-6 py-6">
                        <code className="text-xs bg-slate-50 text-slate-600 px-2 py-1 rounded font-mono">
                          {task.file_name}
                        </code>
                      </td>
                      <td className="px-6 py-6">
                        <span className={`px-3 py-1.5 rounded-full text-[10px] font-bold uppercase tracking-wider border ${getStatusStyle(task.status)}`}>
                          {task.status}
                        </span>
                      </td>
                      <td className="px-6 py-6">
                        <div className="flex flex-col gap-2">
                          <div className="w-32 bg-slate-100 rounded-full h-1.5 overflow-hidden">
                            <div
                              className="bg-slate-900 h-full transition-all duration-500 ease-out"
                              style={{ width: `${task.progress || 0}%` }}
                            />
                          </div>
                          <span className="text-[10px] font-bold text-slate-400">{Math.round(task.progress || 0)}%</span>
                        </div>
                      </td>
                      <td className="px-8 py-6 text-right text-xs text-slate-500 font-mono">
                        {new Date(task.created_at).toLocaleString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
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

export default Tasks;