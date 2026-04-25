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
      case 'completed': return 'text-emerald-500 bg-emerald-50 border-emerald-100';
      case 'failed': return 'text-rose-500 bg-rose-50 border-rose-100';
      case 'processing': return 'text-blue-600 bg-blue-50 border-blue-100 animate-pulse';
      case 'pending': return 'text-slate-400 bg-slate-50 border-slate-100';
      default: return 'text-slate-400 bg-slate-50 border-slate-100';
    }
  };

  if (loading) return (
    <div className="flex h-[60vh] items-center justify-center font-black text-slate-400 uppercase tracking-[0.2em] animate-pulse">
      Установление соединения...
    </div>
  );

  return (
    <div className="max-w-[1200px] mx-auto space-y-10">
      <section className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div className="flex items-center gap-4">
          <div className="h-8 w-[3px] bg-slate-900"></div>
          <div>
            <h1 className="text-2xl font-black text-slate-900 uppercase tracking-tight">Задачи</h1>
            <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Лог и статус синхронизации</p>
          </div>
        </div>

        <div className="relative group">
          <select
            value={selectedServer}
            onChange={(e) => setSelectedServer(e.target.value)}
            className="appearance-none bg-white border border-slate-200 px-6 py-3 pr-12 rounded-xl text-xs font-black uppercase tracking-widest text-slate-700 focus:outline-none focus:ring-2 focus:ring-slate-900 transition-all cursor-pointer"
          >
            <option value="">Все серверы</option>
            {servers.map((server) => (
              <option key={server.id} value={server.id}>{server.name}</option>
            ))}
          </select>
          <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-slate-400">
            ↓
          </div>
        </div>
      </section>

      <div className="bg-white border border-slate-100 rounded-[2rem] overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-50">
                <th className="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400">Сервер</th>
                <th className="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400">Тип</th>
                <th className="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400">Файл</th>
                <th className="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400">Статус</th>
                <th className="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400 w-48">Прогресс</th>
                <th className="px-8 py-5 text-[10px] font-black uppercase tracking-[0.2em] text-slate-400 text-right">Время</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-50">
              {tasks.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-8 py-20 text-center">
                    <div className="text-slate-300 font-bold text-sm uppercase tracking-widest">Задач не обнаружено</div>
                  </td>
                </tr>
              ) : (
                tasks.map((task) => {
                  const server = servers.find((s) => s.id === task.server_id);
                  return (
                    <tr key={task.id} className="hover:bg-slate-50 transition-colors group">
                      <td className="px-8 py-5">
                        <span className="text-xs font-black text-slate-700 uppercase tracking-tight">
                          {server?.name || 'Unknown'}
                        </span>
                      </td>
                      <td className="px-8 py-5">
                        <div className={`inline-flex items-center gap-2 px-2 py-1 rounded text-[9px] font-black uppercase border ${
                          task.direction === 'upload' ? 'bg-blue-50 text-blue-600 border-blue-100' : 'bg-indigo-50 text-indigo-600 border-indigo-100'
                        }`}>
                          {task.direction === 'upload' ? '↑' : '↓'} {task.direction}
                        </div>
                      </td>
                      <td className="px-8 py-5">
                        <code className="text-[11px] font-mono text-slate-500 bg-slate-50 px-2 py-0.5 rounded border border-slate-100">
                          {task.file_name}
                        </code>
                      </td>
                      <td className="px-8 py-5">
                        <span className={`px-3 py-1 rounded-full text-[9px] font-black uppercase border ${getStatusStyle(task.status)}`}>
                          {task.status}
                        </span>
                      </td>
                      <td className="px-8 py-5">
                        <div className="space-y-1.5">
                          <div className="flex justify-between text-[8px] font-black uppercase text-slate-400">
                            <span>{Math.round(task.progress || 0)}%</span>
                          </div>
                          <div className="w-full bg-slate-100 h-1.5 rounded-full overflow-hidden">
                            <div
                              className={`h-full transition-all duration-500 ${
                                task.status === 'failed' ? 'bg-rose-500' : 'bg-slate-900'
                              }`}
                              style={{ width: `${task.progress || 0}%` }}
                            />
                          </div>
                        </div>
                      </td>
                      <td className="px-8 py-5 text-right">
                        <span className="text-[10px] font-bold text-slate-400 tabular-nums">
                          {new Date(task.created_at).toLocaleTimeString()}
                        </span>
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