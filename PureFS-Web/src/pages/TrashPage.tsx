import { useState, useEffect, useCallback } from 'react';
import api from '../api/client';
import type { RecycleBinItem } from '../api';

export default function TrashPage() {
  const [items, setItems] = useState<RecycleBinItem[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchTrash = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<{ items: RecycleBinItem[]; total: number }>('/trash');
      setItems(res.data.items || []);
    } catch (err) {
      console.error('Failed to load trash:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchTrash(); }, [fetchTrash]);

  const handleRestore = async (id: number) => {
    try {
      await api.post(`/trash/${id}/restore`);
      fetchTrash();
    } catch (err) {
      console.error('Failed to restore:', err);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.delete(`/trash/${id}`);
      fetchTrash();
    } catch (err) {
      console.error('Failed to delete:', err);
    }
  };

  const handleEmptyTrash = async () => {
    if (!confirm('永久清空回收站？此操作不可撤销。')) return;
    try {
      await api.delete('/trash');
      fetchTrash();
    } catch (err) {
      console.error('Failed to empty trash:', err);
    }
  };

  const formatSize = (bytes: number) => {
    if (!bytes || bytes === 0) return '-';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
  };

  const formatDate = (date: string) => {
    const d = new Date(date);
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const getExpireDate = (date: string) => {
    const remaining = new Date(date).getTime() - Date.now();
    const days = Math.ceil(remaining / (1000 * 60 * 60 * 24));
    return days <= 0 ? '即将清理' : `${days} 天后清理`;
  };

  return (
    <div className="main-content">
      <div className="header">
        <div className="header-left">
          <h2 style={{ fontSize: 18, fontWeight: 600 }}>🗑️ 回收站</h2>
        </div>
        <div className="header-right">
          {items.length > 0 && (
            <button className="btn" onClick={handleEmptyTrash} style={{ color: 'var(--danger)' }}>
              清空回收站
            </button>
          )}
        </div>
      </div>

      <div className="file-area">
        {loading ? (
          <div className="empty-state"><div className="loading-spinner" style={{ width: 32, height: 32 }} /></div>
        ) : items.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🗑️</div>
            <h3>回收站为空</h3>
            <p>删除的文件会出现在这里，30 天后自动清理</p>
          </div>
        ) : (
          <table className="file-table">
            <thead>
              <tr>
                <th style={{ width: 40 }}></th>
                <th>文件名</th>
                <th style={{ width: 100 }}>大小</th>
                <th style={{ width: 160 }}>删除时间</th>
                <th style={{ width: 100 }}>有效期</th>
                <th style={{ width: 120 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id} className="file-row">
                  <td><span className="file-icon">{item.is_dir ? '📁' : '📄'}</span></td>
                  <td>{item.original_name}</td>
                  <td className="file-size">{item.is_dir ? '-' : formatSize(item.file_size)}</td>
                  <td className="file-date">{formatDate(item.deleted_at)}</td>
                  <td className="file-date" style={{ color: new Date(item.expire_at).getTime() - Date.now() < 86400000 ? 'var(--danger)' : 'var(--text-secondary)' }}>
                    {getExpireDate(item.expire_at)}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn" style={{ fontSize: 12, padding: '2px 8px' }} onClick={() => handleRestore(item.id)}>恢复</button>
                      <button className="btn" style={{ fontSize: 12, padding: '2px 8px', color: 'var(--danger)' }} onClick={() => handleDelete(item.id)}>删除</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
