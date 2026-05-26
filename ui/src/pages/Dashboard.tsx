import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../services/api';
import { WorkerStats, Server, SyncTask } from '../types';

const Dashboard = () => {
  const navigate = useNavigate();
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
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <div className="flex justify-center items-center h-96">
        <div className="w-8 h-8 border-2 border-slate-200 border-t-slate-900 rounded-full animate-spin"></div>
      </div>
    );
  }

  const onlineCount = stats?.workers?.filter(w => w.state === 'running').length || 0;
  const totalTasks = tasks.length;
  const failedTasks = tasks.filter(t => t.status === 'failed').length;
  const pendingTasks = tasks.filter(t => t.status === 'pending' || t.status === 'processing').length;
  const activeServers = servers.filter(s => s.is_active).length;

  return (
    <div className="max-w-7xl mx-auto space-y-10">
      {/* Заголовок */}
      <div className="space-y-1">
        <h1 className="text-4xl font-light tracking-tight text-slate-900">Панель управления</h1>
        <p className="text-slate-500 text-sm uppercase tracking-widest font-semibold">Общая статистика системы</p>
      </div>
      
      {/* Верхние карточки статистики */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <div className="bg-white rounded-[28px] border border-slate-100 p-8 shadow-sm hover:shadow-md transition-all">
          <div className="text-[10px] font-bold text-slate-400 uppercase tracking-[0.15em] mb-3">Всего серверов</div>
          <div className="text-4xl font-light text-slate-900">{servers.length}</div>
        </div>
        
        {/* Кликабельная карточка воркеров */}
        <div 
          onClick={() => navigate('/workers')}
          className="bg-white rounded-[28px] border border-slate-100 p-8 shadow-sm hover:shadow-md transition-all cursor-pointer group"
        >
          <div className="text-[10px] font-bold text-slate-400 uppercase tracking-[0.15em] mb-3 flex items-center justify-between">
            Активных воркеров
            <svg className="w-3 h-3 text-slate-300 group-hover:text-slate-500 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7" />
            </svg>
          </div>
          <div className="text-4xl font-light text-emerald-600">{onlineCount}</div>
          <div className="text-[10px] text-slate-400 mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
            Подробнее →
          </div>
        </div>
        
        <div className="bg-white rounded-[28px] border border-slate-100 p-8 shadow-sm hover:shadow-md transition-all">
          <div className="text-[10px] font-bold text-slate-400 uppercase tracking-[0.15em] mb-3">Всего задач</div>
          <div className="text-4xl font-light text-slate-900">{totalTasks}</div>
        </div>
        
        <div className="bg-white rounded-[28px] border border-slate-100 p-8 shadow-sm hover:shadow-md transition-all">
          <div className="text-[10px] font-bold text-slate-400 uppercase tracking-[0.15em] mb-3">Ошибок</div>
          <div className="text-4xl font-light text-rose-600">{failedTasks}</div>
        </div>
      </div>

      {/* Дополнительные показатели */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-amber-50/50 border border-amber-100 rounded-[28px] p-8 flex justify-between items-center">
          <span className="text-amber-700 font-bold text-sm uppercase tracking-wider">В ожидании</span>
          <span className="text-4xl font-light text-amber-900">{pendingTasks}</span>
        </div>
        <div 
          onClick={() => navigate('/servers')}
          className="bg-blue-50/50 border border-blue-100 rounded-[28px] p-8 flex justify-between items-center cursor-pointer group hover:bg-blue-100/50 transition-all"
        >
          <span className="text-blue-700 font-bold text-sm uppercase tracking-wider">Активных серверов</span>
          <div className="flex items-center gap-3">
            <span className="text-4xl font-light text-blue-900">{activeServers}</span>
            <svg className="w-4 h-4 text-blue-400 group-hover:text-blue-600 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </div>
      </div>
      
      {/* Таблица Статус воркеров */}
      <div className="bg-white rounded-[32px] border border-slate-100 shadow-sm overflow-hidden">
        <div className="px-8 py-6 border-b border-slate-50 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-800">Статус воркеров</h2>
          <button 
            onClick={() => navigate('/workers')}
            className="text-[10px] font-bold text-slate-400 uppercase tracking-widest hover:text-slate-600 transition-colors"
          >
            Все воркеры →
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead className="bg-slate-50/50 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
              <tr>
                <th className="px-8 py-4">Сервер</th>
                <th className="px-6 py-4">Статус</th>
                <th className="px-6 py-4">Последняя синхронизация</th>
                <th className="px-8 py-4 text-right">Успешно / Ошибок</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-50">
              {!stats?.workers || stats.workers.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-8 py-16 text-center text-slate-400 text-sm">
                    Нет активных воркеров. Запустите воркер командой:
                    <code className="block mt-2 text-xs bg-slate-100 p-2 rounded font-mono">
                      ./ssh-sync-service worker
                    </code>
                  </td>
                </tr>
              ) : (
                stats.workers.slice(0, 3).map(worker => (
                  <tr key={worker.server_id} className="hover:bg-slate-50/30 transition-colors">
                    <td className="px-8 py-6">
                      <span className="text-sm font-semibold text-slate-800">{worker.server_name}</span>
                    </td>
                    <td className="px-6 py-6">
                      <span className={`px-3 py-1.5 rounded-full text-[10px] font-bold uppercase tracking-wider border ${
                        worker.state === 'running' ? 'bg-emerald-50 text-emerald-600 border-emerald-100' :
                        worker.state === 'error' ? 'bg-rose-50 text-rose-600 border-rose-100' :
                        'bg-slate-50 text-slate-500 border-slate-100'
                      }`}>
                        {worker.state === 'running' ? '🟢 Работает' : 
                         worker.state === 'error' ? '🔴 Ошибка' : '⚫ Остановлен'}
                      </span>
                    </td>
                    <td className="px-6 py-6 text-sm text-slate-500 font-mono">
                      {worker.last_sync ? new Date(worker.last_sync).toLocaleString('ru-RU') : '—'}
                    </td>
                    <td className="px-8 py-6 text-right">
                      <div className="flex flex-col items-end gap-1">
                        <div className="text-sm font-medium">
                          <span className="text-emerald-500">✓ {worker.sync_count}</span>
                          <span className="text-slate-300 mx-2">/</span>
                          <span className="text-rose-500">✗ {worker.error_count}</span>
                        </div>
                        {worker.last_error && (
                          <div className="text-[10px] text-rose-400 truncate max-w-[200px] italic">
                            {worker.last_error}
                          </div>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {stats?.workers && stats.workers.length > 3 && (
          <div className="px-8 py-4 border-t border-slate-50 text-center">
            <button 
              onClick={() => navigate('/workers')}
              className="text-xs text-slate-500 hover:text-slate-700 transition-colors"
            >
              и ещё {stats.workers.length - 3} воркер(а/ов) → 
            </button>
          </div>
        )}
      </div>

      {/* Таблица Последние задачи */}
      <div className="bg-white rounded-[32px] border border-slate-100 shadow-sm overflow-hidden">
        <div className="px-8 py-6 border-b border-slate-50 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-800">Последние задачи</h2>
          <button 
            onClick={() => navigate('/tasks')}
            className="text-[10px] font-bold text-slate-400 uppercase tracking-widest hover:text-slate-600 transition-colors"
          >
            Все задачи →
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead className="bg-slate-50/50 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
              <tr>
                <th className="px-8 py-4">Файл</th>
                <th className="px-6 py-4">Направление</th>
                <th className="px-6 py-4">Статус</th>
                <th className="px-6 py-4">Прогресс</th>
                <th className="px-8 py-4 text-right">Время</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-50">
              {tasks.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-8 py-16 text-center text-slate-400 text-sm">Нет задач</td>
                </tr>
              ) : (
                tasks.slice(0, 5).map(task => {
                  const server = servers.find(s => s.id === task.server_id);
                  return (
                    <tr key={task.id} className="hover:bg-slate-50/30 transition-colors">
                      <td className="px-8 py-6">
                        <div className="text-xs font-mono text-slate-700 font-bold mb-0.5">{task.file_name}</div>
                        <div className="text-[10px] text-slate-400 uppercase font-bold">{server?.name}</div>
                      </td>
                      <td className="px-6 py-6">
                        <span className={`px-2 py-1 rounded-lg text-[10px] font-bold uppercase tracking-wider ${
                          task.direction === 'upload' ? 'bg-blue-50 text-blue-600' : 'bg-purple-50 text-purple-600'
                        }`}>
                          {task.direction === 'upload' ? '↑ Загрузка' : '↓ Скачивание'}
                        </span>
                      </td>
                      <td className="px-6 py-6">
                        <span className={`px-3 py-1.5 rounded-full text-[10px] font-bold uppercase tracking-wider border ${
                          task.status === 'completed' ? 'bg-emerald-50 text-emerald-600 border-emerald-100' :
                          task.status === 'failed' ? 'bg-rose-50 text-rose-600 border-rose-100' :
                          task.status === 'processing' ? 'bg-blue-50 text-blue-600 border-blue-100 animate-pulse' :
                          'bg-amber-50 text-amber-600 border-amber-100'
                        }`}>
                          {task.status === 'completed' ? '✓ Завершена' :
                           task.status === 'failed' ? '✗ Ошибка' :
                           task.status === 'processing' ? '⟳ Выполняется' : '⏳ Ожидает'}
                        </span>
                      </td>
                      <td className="px-6 py-6">
                        <div className="w-24 bg-slate-100 rounded-full h-1.5 overflow-hidden mb-1.5">
                          <div
                            className="bg-slate-900 h-full transition-all duration-500"
                            style={{ width: `${task.progress || 0}%` }}
                          />
                        </div>
                        <span className="text-[10px] font-bold text-slate-400">{Math.round(task.progress || 0)}%</span>
                      </td>
                      <td className="px-8 py-6 text-right text-xs text-slate-500 font-mono">
                        {new Date(task.created_at).toLocaleTimeString('ru-RU')}
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