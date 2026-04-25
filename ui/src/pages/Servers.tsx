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
      console.error('Ошибка при загрузке:', error);
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
        setEditingServer(null);
        return;
      }

      if (editingServer) await api.updateServer(editingServer.id, data);
      else await api.createServer(data);

      setShowModal(false);
      setEditingServer(null);
      resetForm();
      fetchServers();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Ошибка сохранения');
    }
  };

  const resetForm = () => {
    setFormData({
      name: '', host: '', port: 22, username: '',
      auth_type: 'password', password: '', private_key: '', is_active: true,
    });
  };

  const handleDelete = async (id: string) => {
    if (window.confirm('Вы уверены, что хотите удалить этот сервер?')) {
      try {
        await api.deleteServer(id);
        fetchServers();
      } catch (error) {
        alert('Ошибка при удалении');
      }
    }
  };

  const handleToggleActive = async (server: Server) => {
    try {
      await api.updateServer(server.id, { is_active: !server.is_active });
      fetchServers();
    } catch (error) {
      console.error('Ошибка изменения статуса:', error);
    }
  };

  const openModal = (server?: Server) => {
    if (server) {
      setEditingServer(server);
      setFormData({
        name: server.name, host: server.host, port: server.port,
        username: server.username, auth_type: server.auth_type,
        password: '', private_key: '', is_active: server.is_active,
      });
    } else {
      setEditingServer(null);
      resetForm();
    }
    setShowModal(true);
  };

  if (loading) return (
    <div className="flex h-[60vh] items-center justify-center font-black text-slate-400 uppercase tracking-widest animate-pulse">
      Установление соединения...
    </div>
  );

  return (
    <div className="max-w-[1200px] mx-auto space-y-10">
      {/* Шапка */}
      <section className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div className="flex items-center gap-4">
          <div className="h-8 w-[3px] bg-slate-900"></div>
          <div>
            <h1 className="text-2xl font-black text-slate-900 uppercase tracking-tight">Серверы</h1>
            <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Управление SSH-узлами</p>
          </div>
        </div>
        <button
          onClick={() => openModal()}
          className="bg-slate-900 text-white px-8 py-3 rounded-xl text-xs font-black uppercase tracking-widest hover:bg-blue-600 transition-all shadow-lg shadow-slate-200"
        >
          Добавить сервер
        </button>
      </section>

      {servers.length === 0 ? (
        <div className="bg-white border-2 border-dashed border-slate-100 rounded-[2.5rem] py-24 text-center">
          <div className="text-slate-200 text-6xl font-black mb-4 select-none uppercase tracking-tighter italic">No Nodes</div>
          <p className="text-slate-400 text-xs font-bold uppercase tracking-widest">Список серверов пуст. Начните с добавления первого узла.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {servers.map((server) => (
            <div key={server.id} className="bg-white border border-slate-100 rounded-3xl p-6 shadow-sm hover:shadow-xl hover:shadow-slate-200/50 transition-all group border-b-4 border-b-slate-100 hover:border-b-blue-500">
              <div className="flex justify-between items-start mb-6">
                <div className={`px-3 py-1 rounded-full text-[9px] font-black uppercase tracking-tighter ${
                  server.is_active ? 'bg-emerald-50 text-emerald-600 border border-emerald-100' : 'bg-slate-50 text-slate-400 border border-slate-100'
                }`}>
                  {server.is_active ? 'Активен' : 'Пауза'}
                </div>
                <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button onClick={() => openModal(server)} className="text-[10px] font-bold text-blue-500 uppercase hover:underline">Edit</button>
                  <button onClick={() => handleDelete(server.id)} className="text-[10px] font-bold text-rose-500 uppercase hover:underline">Del</button>
                </div>
              </div>

              <h3 className="text-xl font-black text-slate-900 mb-4 truncate uppercase tracking-tight">{server.name}</h3>
              
              <div className="space-y-3 mb-8">
                <div className="flex justify-between items-center text-[11px]">
                  <span className="font-bold text-slate-300 uppercase tracking-widest">Адрес</span>
                  <code className="bg-slate-50 px-2 py-1 rounded font-mono text-slate-600">{server.host}:{server.port}</code>
                </div>
                <div className="flex justify-between items-center text-[11px]">
                  <span className="font-bold text-slate-300 uppercase tracking-widest">Пользователь</span>
                  <span className="font-bold text-slate-700">{server.username}</span>
                </div>
                <div className="flex justify-between items-center text-[11px]">
                  <span className="font-bold text-slate-300 uppercase tracking-widest">Авторизация</span>
                  <span className="font-bold text-slate-700 uppercase">{server.auth_type === 'key' ? 'SSH Key' : 'Password'}</span>
                </div>
              </div>

              <div className="pt-4 border-t border-slate-50 flex items-center justify-between">
                <div className="text-[9px] font-bold text-slate-400 uppercase tracking-tighter">
                  {server.last_seen ? `Видели: ${new Date(server.last_seen).toLocaleDateString()}` : 'Нет данных'}
                </div>
                <button
                  onClick={() => handleToggleActive(server)}
                  className={`w-10 h-5 rounded-full relative transition-colors ${server.is_active ? 'bg-blue-600' : 'bg-slate-200'}`}
                >
                  <div className={`absolute top-1 w-3 h-3 bg-white rounded-full transition-all ${server.is_active ? 'left-6' : 'left-1'}`}></div>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-sm flex items-center justify-center z-[100] p-4">
          <div className="bg-white rounded-[2rem] shadow-2xl w-full max-w-lg max-h-[90vh] overflow-y-auto">
            <div className="p-8">
              <header className="flex justify-between items-center mb-8">
                <div>
                  <h2 className="text-xl font-black uppercase tracking-tight text-slate-900">
                    {editingServer ? 'Настройка узла' : 'Новый сервер'}
                  </h2>
                  <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest italic">Конфигурация SSH соединения</p>
                </div>
                <button 
                  onClick={() => setShowModal(false)}
                  className="w-8 h-8 rounded-full bg-slate-50 flex items-center justify-center text-slate-400 hover:bg-slate-900 hover:text-white transition-all"
                >
                  ✕
                </button>
              </header>

              <form onSubmit={handleSubmit} className="space-y-5">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="md:col-span-2">
                    <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Название узла</label>
                    <input
                      type="text" required value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-bold text-sm"
                      placeholder="Production_1"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Хост</label>
                    <input
                      type="text" required value={formData.host}
                      onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                      className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-bold text-sm"
                      placeholder="192.168.1.1"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Порт</label>
                    <input
                      type="number" required value={formData.port}
                      onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                      className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-bold text-sm"
                      placeholder="22"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Логин</label>
                    <input
                      type="text" required value={formData.username}
                      onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                      className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-bold text-sm"
                      placeholder="root"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Тип входа</label>
                    <select
                      value={formData.auth_type}
                      onChange={(e) => setFormData({ ...formData, auth_type: e.target.value as 'password' | 'key' })}
                      className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-bold text-sm appearance-none"
                    >
                      <option value="password">Пароль</option>
                      <option value="key">SSH Ключ</option>
                    </select>
                  </div>
                </div>

                <div className="pt-2">
                  {formData.auth_type === 'password' ? (
                    <div>
                      <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Пароль</label>
                      <input
                        type="password" required={!editingServer} value={formData.password}
                        onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                        className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-bold text-sm"
                        placeholder={editingServer ? '•••••••• (оставьте пустым)' : 'Введите пароль'}
                      />
                    </div>
                  ) : (
                    <div>
                      <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-2">Private Key</label>
                      <textarea
                        required={!editingServer} value={formData.private_key}
                        onChange={(e) => setFormData({ ...formData, private_key: e.target.value })}
                        className="w-full px-5 py-3 bg-slate-50 border-none rounded-xl focus:ring-2 focus:ring-blue-500 font-mono text-[11px] h-32"
                        placeholder="-----BEGIN RSA PRIVATE KEY-----"
                      />
                    </div>
                  )}
                </div>

                <div className="pt-6 flex gap-3">
                  <button
                    type="submit"
                    className="flex-1 bg-slate-900 text-white py-4 rounded-2xl text-xs font-black uppercase tracking-[0.2em] hover:bg-blue-600 transition-all shadow-xl shadow-blue-100"
                  >
                    {editingServer ? 'Обновить данные' : 'Создать узел'}
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