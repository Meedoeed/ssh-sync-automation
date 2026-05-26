import { useEffect, useState } from 'react';
import { api } from '../services/api';
import { WorkerStats, Worker } from '../types';

const Workers = () => {
  const [stats, setStats] = useState<WorkerStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);

  useEffect(() => {
    fetchWorkers();
    let interval: NodeJS.Timeout;
    if (autoRefresh) {
      interval = setInterval(fetchWorkers, 5000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [autoRefresh]);

  const fetchWorkers = async () => {
    try {
      const res = await api.getWorkerStats();
      setStats(res.data);
    } catch (error) {
      console.error('Ошибка загрузки воркеров:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-96">
        <div className="w-8 h-8 border-2 border-slate-200 border-t-slate-900 rounded-full animate-spin"></div>
      </div>
    );
  }

  // Безопасное получение значений (stats может быть null)
  const onlineCount = stats?.workers?.filter(w => w.state === 'running').length || 0;
  const errorCount = stats?.workers?.filter(w => w.state === 'error').length || 0;
  const offlineCount = stats?.workers?.filter(w => w.state === 'stopped').length || 0;
  const totalWorkers = stats?.total_workers || 0;
  const workersList = stats?.workers || [];

  return (
    <div className="max-w-7xl mx-auto space-y-10">
      <div className="flex justify-between items-end">
        <div className="space-y-1">
          <h1 className="text-4xl font-light tracking-tight text-slate-900">Воркеры</h1>
          <p className="text-slate-500 text-sm uppercase tracking-widest font-semibold">
            Мониторинг исполнителей задач
          </p>
        </div>
        
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className={`w-2 h-2 rounded-full transition-colors ${autoRefresh ? 'bg-emerald-500' : 'bg-slate-300'}`}></div>
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className="text-xs font-medium text-slate-500 hover:text-slate-700 transition-colors"
            >
              {autoRefresh ? 'Автообновление включено' : 'Автообновление выключено'}
            </button>
          </div>
          <button
            onClick={fetchWorkers}
            className="px-4 py-2 bg-slate-100 text-slate-600 rounded-xl text-sm font-medium hover:bg-slate-200 transition-all"
          >
            Обновить
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        {[
          { label: 'Всего воркеров', value: totalWorkers, color: 'text-slate-900' },
          { label: 'Активных', value: onlineCount, color: 'text-emerald-600' },
          { label: 'С ошибкой', value: errorCount, color: 'text-rose-600' },
          { label: 'Остановлены', value: offlineCount, color: 'text-slate-400' }
        ].map((item, idx) => (
          <div key={idx} className="bg-white rounded-[28px] border border-slate-100 p-8 shadow-sm hover:shadow-md transition-all">
            <div className="text-[10px] font-bold text-slate-400 uppercase tracking-[0.15em] mb-3">{item.label}</div>
            <div className={`text-4xl font-light ${item.color}`}>{item.value}</div>
          </div>
        ))}
      </div>

      <div className="bg-white rounded-[32px] border border-slate-100 shadow-sm overflow-hidden">
        <div className="px-8 py-6 border-b border-slate-50 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-800">Список воркеров</h2>
          <div className="flex gap-2">
            <span className="text-[10px] text-slate-400">Heartbeat: каждые 10 сек</span>
          </div>
        </div>
        
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead className="bg-slate-50/50 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
              <tr>
                <th className="px-8 py-4">ID Воркера</th>
                <th className="px-6 py-4">Статус</th>
                <th className="px-6 py-4">Сервер</th>
                <th className="px-6 py-4">Последний heartbeat</th>
                <th className="px-8 py-4 text-right">Успешно / Ошибок</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-50">
              {workersList.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-8 py-16 text-center text-slate-400 text-sm">
                    Нет активных воркеров. Запустите воркер командой:
                    <code className="block mt-2 text-xs bg-slate-100 p-2 rounded font-mono">
                      ./ssh-sync-service worker
                    </code>
                  </td>
                </tr>
              ) : (
                workersList.map((worker) => (
                  <tr key={worker.server_id} className="hover:bg-slate-50/30 transition-colors group">
                    <td className="px-8 py-6">
                      <div className="flex items-center gap-2">
                        <div className={`w-2 h-2 rounded-full ${
                          worker.state === 'running' ? 'bg-emerald-500 animate-pulse' :
                          worker.state === 'error' ? 'bg-rose-500' : 'bg-slate-400'
                        }`}></div>
                        <code className="text-xs font-mono text-slate-600 bg-slate-50 px-2 py-1 rounded">
                          {worker.server_id.slice(0, 12)}...
                        </code>
                      </div>
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
                    <td className="px-6 py-6">
                      <span className="text-sm font-medium text-slate-700">{worker.server_name || '—'}</span>
                    </td>
                    <td className="px-6 py-6">
                      <div className="flex flex-col">
                        <span className="text-sm text-slate-600 font-mono">
                          {worker.last_sync ? new Date(worker.last_sync).toLocaleString('ru-RU') : '—'}
                        </span>
                        {worker.last_sync && (
                          <span className="text-[10px] text-slate-400 mt-0.5">
                            {Math.floor((Date.now() - new Date(worker.last_sync).getTime()) / 1000)} сек назад
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-8 py-6 text-right">
                      <div className="flex flex-col items-end gap-1">
                        <div className="text-sm font-medium">
                          <span className="text-emerald-500">✓ {worker.sync_count}</span>
                          <span className="text-slate-300 mx-2">/</span>
                          <span className="text-rose-500">✗ {worker.error_count}</span>
                        </div>
                        {worker.last_error && (
                          <div className="text-[10px] text-rose-400 truncate max-w-[200px] italic group-hover:max-w-none group-hover:whitespace-normal">
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
      </div>

      <div className="bg-slate-50/50 rounded-[32px] border border-slate-100 p-8">
        <h3 className="text-sm font-semibold text-slate-700 mb-4">О воркерах</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 text-sm">
          <div className="flex items-start gap-3">
            <div className="w-8 h-8 bg-emerald-100 rounded-xl flex items-center justify-center text-emerald-600">🟢</div>
            <div>
              <div className="font-medium text-slate-800 mb-1">Активный воркер</div>
              <div className="text-xs text-slate-500">Регулярно отправляет heartbeat и выполняет задачи</div>
            </div>
          </div>
          <div className="flex items-start gap-3">
            <div className="w-8 h-8 bg-rose-100 rounded-xl flex items-center justify-center text-rose-600">🔴</div>
            <div>
              <div className="font-medium text-slate-800 mb-1">Ошибка</div>
              <div className="text-xs text-slate-500">Воркер не может подключиться или выполнить задачу</div>
            </div>
          </div>
          <div className="flex items-start gap-3">
            <div className="w-8 h-8 bg-slate-200 rounded-xl flex items-center justify-center text-slate-500">⚫</div>
            <div>
              <div className="font-medium text-slate-800 mb-1">Остановлен</div>
              <div className="text-xs text-slate-500">Воркер не отвечает на heartbeat (скорее всего остановлен)</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Workers;