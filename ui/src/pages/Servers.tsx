import { useEffect, useState } from 'react';
import { api } from '../services/api';
import { Server } from '../types';

const Servers = () => {
  const [servers, setServers] = useState<Server[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingServer, setEditingServer] = useState<Server | null>(null);
  const [formData, setFormData] = useState({
    name: '',
    host: '',
    port: 22,
    username: '',
    auth_type: 'password' as 'password' | 'key',
    password: '',
    private_key: '',
    is_active: true,
  });

  useEffect(() => {
    fetchServers();
  }, []);

  const fetchServers = async () => {
    try {
      const res = await api.getServers();
      setServers(res.data);
    } catch (error) {
      console.error('Ошибка загрузки серверов:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const data: any = {};
      if (editingServer) {
        if (formData.name !== editingServer.name) data.name = formData.name;
        if (formData.host !== editingServer.host) data.host = formData.host;
        if (formData.port !== editingServer.port) data.port = formData.port;
        if (formData.username !== editingServer.username) data.username = formData.username;
        if (formData.auth_type !== editingServer.auth_type) data.auth_type = formData.auth_type;
        if (formData.is_active !== editingServer.is_active) data.is_active = formData.is_active;
        if (formData.auth_type === 'password' && formData.password) data.password = formData.password;
        if (formData.auth_type === 'key' && formData.private_key) data.private_key = formData.private_key;
      } else {
        Object.assign(data, { ...formData });
        if (formData.auth_type === 'password') delete data.private_key;
        else delete data.password;
      }

      if (editingServer && Object.keys(data).length === 0) {
        setShowModal(false);
        return;
      }

      editingServer ? await api.updateServer(editingServer.id, data) : await api.createServer(data);
      setShowModal(false);
      resetForm();
      fetchServers();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Ошибка сохранения');
    }
  };

  const resetForm = () => {
    setFormData({ name: '', host: '', port: 22, username: '', auth_type: 'password', password: '', private_key: '', is_active: true });
    setEditingServer(null);
  };

  const handleDelete = async (id: string) => {
    if (window.confirm('Удалить этот сервер из системы?')) {
      try {
        await api.deleteServer(id);
        fetchServers();
      } catch (error) {
        console.error(error);
      }
    }
  };

  const handleToggleActive = async (server: Server) => {
    try {
      await api.updateServer(server.id, { is_active: !server.is_active });
      fetchServers();
    } catch (error) {
      console.error(error);
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
      <div className="flex justify-between items-end">
        <div className="space-y-1">
          <h1 className="text-4xl font-light tracking-tight text-slate-900">Узлы сети</h1>
          <p className="text-slate-500 text-sm uppercase tracking-widest font-semibold">Управление SSH-соединениями</p>
        </div>
        <button
          onClick={() => { resetForm(); setShowModal(true); }}
          className="bg-slate-900 text-white px-6 py-3 rounded-2xl text-sm font-bold tracking-wide hover:bg-slate-800 transition-all shadow-lg shadow-slate-200 active:scale-95"
        >
          Добавить узел
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {servers.length === 0 ? (
          <div className="col-span-full py-20 text-center bg-white rounded-3xl border border-dashed border-slate-200">
            <span className="text-slate-400 font-medium">Список серверов пуст</span>
          </div>
        ) : (
          servers.map((server) => (
            <div key={server.id} className="bg-white rounded-3xl border border-slate-100 p-6 shadow-sm hover:shadow-xl hover:translate-y-[-4px] transition-all duration-300 group">
              <div className="flex justify-between items-start mb-6">
                <div className="space-y-1">
                  <h3 className="text-xl font-semibold text-slate-800">{server.name}</h3>
                  <code className="text-[11px] bg-slate-50 text-slate-500 px-2 py-1 rounded tracking-tight">
                    {server.host}:{server.port}
                  </code>
                </div>
                <button
                  onClick={() => handleToggleActive(server)}
                  className={`w-12 h-6 rounded-full transition-colors relative ${server.is_active ? 'bg-emerald-500' : 'bg-slate-200'}`}
                >
                  <div className={`absolute top-1 w-4 h-4 bg-white rounded-full transition-all ${server.is_active ? 'left-7' : 'left-1'}`}></div>
                </button>
              </div>

              <div className="space-y-4 mb-8">
                <div className="flex justify-between text-sm">
                  <span className="text-slate-400">Пользователь</span>
                  <span className="text-slate-700 font-medium">{server.username}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-slate-400">Авторизация</span>
                  <span className="text-slate-700 font-medium">{server.auth_type === 'password' ? 'Пароль' : 'RSA Ключ'}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-slate-400">Активность</span>
                  <span className="text-slate-500 font-light italic">
                    {server.last_seen ? new Date(server.last_seen).toLocaleDateString() : 'Нет данных'}
                  </span>
                </div>
              </div>

              <div className="flex gap-2 pt-4 border-t border-slate-50">
                <button
                  onClick={() => { setEditingServer(server); setFormData({ ...server, password: '', private_key: '' }); setShowModal(true); }}
                  className="flex-1 py-2 text-[11px] font-bold uppercase tracking-wider text-slate-400 hover:text-slate-900 hover:bg-slate-50 rounded-xl transition-all"
                >
                  Настроить
                </button>
                <button
                  onClick={() => handleDelete(server.id)}
                  className="px-4 py-2 text-[11px] font-bold uppercase tracking-wider text-rose-300 hover:text-rose-600 hover:bg-rose-50 rounded-xl transition-all"
                >
                  Удалить
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {showModal && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-[32px] shadow-2xl w-full max-w-lg overflow-hidden animate-in fade-in zoom-in duration-200">
            <div className="p-10">
              <div className="flex flex-col gap-2 mb-8">
                <h2 className="text-2xl font-semibold text-slate-900">
                  {editingServer ? 'Параметры узла' : 'Новое подключение'}
                </h2>
                <p className="text-sm text-slate-400">Заполните данные для SSH доступа</p>
              </div>
              
              <form onSubmit={handleSubmit} className="space-y-6">
                <div className="grid grid-cols-2 gap-4">
                  <div className="col-span-2">
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">Название метки</label>
                    <input
                      type="text" required value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all placeholder:text-slate-300 text-sm"
                      placeholder="Production Main"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">IP / Хост</label>
                    <input
                      type="text" required value={formData.host}
                      onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all text-sm"
                      placeholder="1.1.1.1"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">Порт</label>
                    <input
                      type="number" required value={formData.port}
                      onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all text-sm"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="col-span-1">
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">User</label>
                    <input
                      type="text" required value={formData.username}
                      onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all text-sm"
                    />
                  </div>
                  <div className="col-span-1">
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">Метод</label>
                    <select
                      value={formData.auth_type}
                      onChange={(e) => setFormData({ ...formData, auth_type: e.target.value as 'password' | 'key' })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all text-sm appearance-none"
                    >
                      <option value="password">Пароль</option>
                      <option value="key">RSA Ключ</option>
                    </select>
                  </div>
                </div>

                {formData.auth_type === 'password' ? (
                  <div>
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">Пароль</label>
                    <input
                      type="password" required={!editingServer}
                      value={formData.password}
                      onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all text-sm"
                      placeholder={editingServer ? "••••••••" : ""}
                    />
                  </div>
                ) : (
                  <div>
                    <label className="block text-[11px] font-bold text-slate-400 uppercase tracking-widest mb-2 px-1">Private Key Content</label>
                    <textarea
                      required={!editingServer}
                      value={formData.private_key}
                      onChange={(e) => setFormData({ ...formData, private_key: e.target.value })}
                      className="w-full px-4 py-3 bg-slate-50 border-none rounded-2xl focus:ring-2 focus:ring-slate-900 transition-all font-mono text-[10px] leading-tight"
                      rows={5}
                    />
                  </div>
                )}

                <div className="flex gap-3 pt-6">
                  <button
                    type="button"
                    onClick={() => setShowModal(false)}
                    className="flex-1 py-4 text-sm font-bold text-slate-400 hover:text-slate-600 transition-colors"
                  >
                    Отмена
                  </button>
                  <button
                    type="submit"
                    className="flex-[2] py-4 bg-slate-900 text-white rounded-2xl text-sm font-bold shadow-lg shadow-slate-100 hover:bg-slate-800 transition-all active:scale-95"
                  >
                    {editingServer ? 'Обновить данные' : 'Подключить сервер'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Servers;