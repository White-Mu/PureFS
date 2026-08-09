import { useState, useEffect } from 'react';
import { shares } from '../api';
import type { Share } from '../api';

const API_BASE = import.meta.env.VITE_API_BASE || window.location.origin;

export default function SharesPage() {
  const [list, setList] = useState<Share[]>([]);
  const [loading, setLoading] = useState(true);
  const [copiedId, setCopiedId] = useState<number | null>(null);

  const fetchShares = () => {
    setLoading(true);
    shares.list()
      .then(setList)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchShares(); }, []);

  const handleCopy = (s: Share) => {
    const url = `${API_BASE}/share/${s.token}`;
    navigator.clipboard.writeText(url);
    setCopiedId(s.id);
    setTimeout(() => setCopiedId(null), 1500);
  };

  const handleDeactivate = async (id: number) => {
    try {
      await shares.deactivate(id);
      setList(prev => prev.map(s => s.id === id ? { ...s, is_active: false } : s));
    } catch {}
  };

  const formatDate = (d: string) =>
    new Date(d).toLocaleDateString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });

  return (
    <div className="main-content">
      <div className="header">
        <div className="header-left">
          <span style={{ fontWeight: 600, fontSize: 16 }}>Shares</span>
          <span style={{ fontSize: 13, color: 'var(--text-tertiary)', marginLeft: 8 }}>
            {list.length} links
          </span>
        </div>
        <div className="header-right">
          <button className="btn" onClick={fetchShares}>↻ Refresh</button>
        </div>
      </div>
      <div className="file-area">
        {loading ? (
          <div className="empty-state"><div className="loading-spinner" /></div>
        ) : list.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🔗</div>
            <h3>No shares yet</h3>
            <p>Right-click a file and select "Share" to create a link</p>
          </div>
        ) : (
          <table className="file-table">
            <thead>
              <tr>
                <th>File</th>
                <th>Created</th>
                <th>Access</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {list.map(s => (
                <tr key={s.id} className="file-row">
                  <td style={{ fontSize: 14, fontWeight: 500 }}>{s.file_name || `#${s.file_id}`}</td>
                  <td className="file-date">{formatDate(s.created_at)}</td>
                  <td style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                    {s.access_count}{s.max_accesses ? ` / ${s.max_accesses}` : ''}
                  </td>
                  <td>
                    <span style={{
                      fontSize: 13, fontWeight: 500,
                      color: s.is_active ? 'var(--success)' : 'var(--text-tertiary)',
                    }}>
                      {s.is_active ? 'Active' : 'Disabled'}
                    </span>
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button className="btn" style={{ padding: '4px 10px', fontSize: 12, marginRight: 6 }}
                      onClick={() => handleCopy(s)}>
                      {copiedId === s.id ? 'Copied!' : 'Copy Link'}
                    </button>
                    {s.is_active && (
                      <button className="btn btn-danger" style={{ padding: '4px 10px', fontSize: 12 }}
                        onClick={() => handleDeactivate(s.id)}>
                        Disable
                      </button>
                    )}
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
