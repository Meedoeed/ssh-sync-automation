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
        console.error('Ошибка:', error);
      } finally {
        setLoading(false);
      }
    };
    
    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, []);

  if (loading) return (
    <div className="flex h-[60vh] items-center justify-center">
      <div className="flex flex-col items-center gap-2">
        <div className="w-8 h-8 border-2 border-slate-900 border-t-transparent rounded-full animate-spin"></div>
        <div className="text-[10px] font-bold uppercase tracking-widest text-slate-400">Синхронизация данных</div>
      </div>
    </div>
  );

  const runningWorkers = stats?.workers.filter(w => w.state === 'running') || [];

  return (
    <div className="max-w-[1200px] mx-auto space-y-10">
      {/* Заголовок — теперь аккуратный */}
      <section className="flex items-center gap-4">
        <div className="h-8 w-[3px] bg-slate-900"></div>
        <div>
          <h1 className="text-2xl font-black text-slate-900 uppercase tracking-tight">
            Панель управления
          </h1>
          <div className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">
            Общая статистика системы
          </div>
        </div>
      </section>

      {/* Верхние показатели */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: 'Серверы', value: servers.length, color: 'text-slate-900' },
          { label: 'В сети', value: runningWorkers.length, color: 'text-emerald-500' },
          { label: 'Задачи', value: tasks.length, color: 'text-blue-600' },
          { label: 'Ошибки', value: tasks.filter(t => t.status === 'failed').length, color: 'text-rose-500' },
        ].map((stat, i) => (
          <div key={i} className="bg-white border border-slate-100 p-6 rounded-xl shadow-sm">
            <div className="text-[9px] font-black uppercase tracking-widest text-slate-400 mb-1">{stat.label}</div>
            <div className={`text-2xl font-black ${stat.color}`}>{stat.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Секция Воркеров */}
        <div className="lg:col-span-7 space-y-6">
          <h2 className="text-xs font-black uppercase tracking-widest text-slate-400 border-b border-slate-100 pb-3">
            Состояние воркеров
          </h2>

          {!stats?.workers || stats.workers.length === 0 ? (
            <div className="bg-slate-50/50 border border-slate-100 rounded-2xl py-16 px-6 text-center">
              <div className="text-slate-400 font-bold text-sm mb-1">Список серверов пуст</div>
              <p className="text-slate-400 text-[11px] uppercase tracking-tighter">Добавьте свой первый сервер в разделе «Серверы», чтобы запустить синхронизацию</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-3">
              {stats.workers.map(worker => (
                <div key={worker.server_id} className="bg-white border border-slate-100 p-5 rounded-xl flex items-center justify-between group hover:border-slate-300 transition-colors">
                  <div className="flex items-center gap-4">
                    <div className={`w-2 h-2 rounded-full ${worker.state === 'running' ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]' : 'bg-slate-300'}`}></div>
                    <div>
                      <div className="text-sm font-bold text-slate-800 uppercase tracking-tight">{worker.server_name}</div>
                      <div className="text-[10px] font-medium text-slate-400 tabular-nums">
                        {worker.last_sync ? `Последняя синхронизация: ${new Date(worker.last_sync).toLocaleTimeString()}` : 'Ожидание запуска'}
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-6">
                    <div className="text-center">
                      <div className="text-[8px] font-black text-slate-300 uppercase">Success</div>
                      <div className="text-xs font-bold text-slate-600">{worker.sync_count}</div>
                    </div>
                    <div className="text-center">
                      <div className="text-[8px] font-black text-slate-300 uppercase">Errors</div>
                      <div className="text-xs font-bold text-rose-500">{worker.error_count}</div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Секция Задач */}
        <div className="lg:col-span-5 space-y-6">
          <h2 className="text-xs font-black uppercase tracking-widest text-slate-400 border-b border-slate-100 pb-3">
            Активные задачи
          </h2>
          <div className="bg-white border border-slate-100 rounded-2xl overflow-hidden shadow-sm min-h-[300px]">
            {tasks.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full py-20 px-10 text-center">
                <div className="text-slate-400 font-bold text-sm mb-1">Задачи отсутствуют</div>
                <p className="text-slate-400 text-[11px] uppercase tracking-tighter">На данный момент нет файлов в очереди на обработку</p>
              </div>
            ) : (
              <div className="divide-y divide-slate-50">
                {tasks.slice(0, 6).map(task => (
                  <div key={task.id} className="p-4 hover:bg-slate-50 transition-colors">
                    <div className="flex justify-between items-center mb-2">
                      <span className="text-[11px] font-bold text-slate-700 truncate max-w-[180px]">{task.file_name}</span>
                      <span className={`text-[9px] font-black uppercase px-2 py-0.5 rounded ${
                        task.status === 'completed' ? 'bg-emerald-50 text-emerald-600' : 'bg-blue-50 text-blue-600'
                      }`}>
                        {task.status === 'completed' ? 'Завершено' : 'В процессе'}
                      </span>
                    </div>
                    <div className="h-1 bg-slate-100 rounded-full overflow-hidden">
                      <div 
                        className="bg-slate-900 h-full transition-all duration-500"
                        style={{ width: `${task.progress || 0}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;