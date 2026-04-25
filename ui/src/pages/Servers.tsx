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
      console.error('Failed to fetch servers:', error);
    } finally {
      setLoading(false);
    }
  };

 const handleSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  try {
    const data: any = {};
    
    // Отправляем только измененные поля при редактировании
    if (editingServer) {
      if (formData.name !== editingServer.name) data.name = formData.name;
      if (formData.host !== editingServer.host) data.host = formData.host;
      if (formData.port !== editingServer.port) data.port = formData.port;
      if (formData.username !== editingServer.username) data.username = formData.username;
      if (formData.auth_type !== editingServer.auth_type) data.auth_type = formData.auth_type;
      if (formData.is_active !== editingServer.is_active) data.is_active = formData.is_active;
      
      // Для пароля/ключа отправляем только если ввели новые
      if (formData.auth_type === 'password' && formData.password) {
        data.password = formData.password;
      }
      if (formData.auth_type === 'key' && formData.private_key) {
        data.private_key = formData.private_key;
      }
    } else {
      // При создании отправляем все поля
      data.name = formData.name;
      data.host = formData.host;
      data.port = formData.port;
      data.username = formData.username;
      data.auth_type = formData.auth_type;
      data.is_active = formData.is_active;
      
      if (formData.auth_type === 'password') {
        data.password = formData.password;
      } else {
        data.private_key = formData.private_key;
      }
    }
    
    // Если нет полей для обновления, показываем сообщение
    if (editingServer && Object.keys(data).length === 0) {
      alert('No changes to update');
      setShowModal(false);
      setEditingServer(null);
      return;
    }
    
    if (editingServer) {
      await api.updateServer(editingServer.id, data);
    } else {
      await api.createServer(data);
    }
    
    setShowModal(false);
    setEditingServer(null);
    resetForm();
    fetchServers();
  } catch (error: any) {
    console.error('Failed to save server:', error);
    const errorMsg = error.response?.data?.error || 'Failed to save server';
    alert(errorMsg);
  }
};

const resetForm = () => {
  setFormData({
    name: '',
    host: '',
    port: 22,
    username: '',
    auth_type: 'password',
    password: '',
    private_key: '',
    is_active: true,
  });
};

  const handleDelete = async (id: string) => {
    if (window.confirm('Are you sure you want to delete this server?')) {
      try {
        await api.deleteServer(id);
        fetchServers();
      } catch (error) {
        console.error('Failed to delete server:', error);
        alert('Failed to delete server');
      }
    }
  };

  const handleToggleActive = async (server: Server) => {
    try {
      await api.updateServer(server.id, { is_active: !server.is_active });
      fetchServers();
    } catch (error) {
      console.error('Failed to toggle server status:', error);
    }
  };

  const openModal = (server?: Server) => {
    if (server) {
      setEditingServer(server);
      setFormData({
        name: server.name,
        host: server.host,
        port: server.port,
        username: server.username,
        auth_type: server.auth_type,
        password: '',
        private_key: '',
        is_active: server.is_active,
      });
    } else {
      setEditingServer(null);
      setFormData({
        name: '',
        host: '',
        port: 22,
        username: '',
        auth_type: 'password',
        password: '',
        private_key: '',
        is_active: true,
      });
    }
    setShowModal(true);
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-500">Loading servers...</div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Servers</h1>
        <button
          onClick={() => openModal()}
          className="bg-blue-500 text-white px-4 py-2 rounded-lg hover:bg-blue-600 transition-colors"
        >
          + Add Server
        </button>
      </div>

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Name</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Host:Port</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Username</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Auth</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Status</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Last Seen</th>
              <th className="p-3 text-left text-sm font-semibold text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody>
            {servers.length === 0 ? (
              <tr>
                <td colSpan={7} className="p-8 text-center text-gray-500">
                  No servers configured. Click "Add Server" to get started.
                </td>
              </tr>
            ) : (
              servers.map((server) => (
                <tr key={server.id} className="border-t hover:bg-gray-50">
                  <td className="p-3 font-medium">{server.name}</td>
                  <td className="p-3">
                    <code className="text-sm bg-gray-100 px-2 py-1 rounded">
                      {server.host}:{server.port}
                    </code>
                  </td>
                  <td className="p-3">{server.username}</td>
                  <td className="p-3">
                    <span className="px-2 py-1 rounded text-xs bg-gray-100">
                      {server.auth_type}
                    </span>
                  </td>
                  <td className="p-3">
                    <button
                      onClick={() => handleToggleActive(server)}
                      className={`px-2 py-1 rounded text-xs font-medium transition-colors ${
                        server.is_active
                          ? 'bg-green-100 text-green-800 hover:bg-green-200'
                          : 'bg-gray-100 text-gray-800 hover:bg-gray-200'
                      }`}
                    >
                      {server.is_active ? 'Active' : 'Inactive'}
                    </button>
                  </td>
                  <td className="p-3 text-sm text-gray-500">
                    {server.last_seen ? new Date(server.last_seen).toLocaleString() : '-'}
                  </td>
                  <td className="p-3">
                    <button
                      onClick={() => openModal(server)}
                      className="text-blue-500 hover:text-blue-700 mr-3 text-sm"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(server.id)}
                      className="text-red-500 hover:text-red-700 text-sm"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-md max-h-[90vh] overflow-y-auto">
            <div className="p-6">
              <h2 className="text-xl font-bold mb-4">
                {editingServer ? 'Edit Server' : 'Add Server'}
              </h2>
              <form onSubmit={handleSubmit}>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Name *
                    </label>
                    <input
                      type="text"
                      required
                      value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="my-server-1"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Host *
                    </label>
                    <input
                      type="text"
                      required
                      value={formData.host}
                      onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="192.168.1.100 or host.docker.internal"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Port *
                    </label>
                    <input
                      type="number"
                      required
                      value={formData.port}
                      onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="22"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Username *
                    </label>
                    <input
                      type="text"
                      required
                      value={formData.username}
                      onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="root"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Authentication Type *
                    </label>
                    <select
                      value={formData.auth_type}
                      onChange={(e) => setFormData({ ...formData, auth_type: e.target.value as 'password' | 'key' })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="password">Password</option>
                      <option value="key">SSH Key</option>
                    </select>
                  </div>
                  
                  {formData.auth_type === 'password' ? (
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        Password *
                      </label>
                      <input
                        type="password"
                        required={!editingServer}
                        value={formData.password}
                        onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder={editingServer ? 'Leave blank to keep unchanged' : 'Enter password'}
                      />
                    </div>
                  ) : (
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        Private Key *
                      </label>
                      <textarea
                        required={!editingServer}
                        value={formData.private_key}
                        onChange={(e) => setFormData({ ...formData, private_key: e.target.value })}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                        rows={4}
                        placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                      />
                    </div>
                  )}
                  
                  <div className="flex items-center">
                    <input
                      type="checkbox"
                      id="is_active"
                      checked={formData.is_active}
                      onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                      className="w-4 h-4 text-blue-500 border-gray-300 rounded focus:ring-blue-500"
                    />
                    <label htmlFor="is_active" className="ml-2 text-sm text-gray-700">
                      Active (start synchronization immediately)
                    </label>
                  </div>
                </div>
                
                <div className="flex justify-end gap-3 mt-6">
                  <button
                    type="button"
                    onClick={() => {
                      setShowModal(false);
                      setEditingServer(null);
                    }}
                    className="px-4 py-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
                  >
                    {editingServer ? 'Update' : 'Create'}
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