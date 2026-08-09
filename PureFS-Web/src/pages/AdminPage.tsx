import { useState, useEffect } from 'react';
import { useAuthStore } from '../store';
import { admin } from '../api';
import type { User, AuditLogEntry, Permission } from '../api';

export default function AdminPage() {
  const user = useAuthStore((s) => s.user);
  const [tab, setTab] = useState<'audit' | 'users' | 'permissions'>('audit');
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (user?.role !== 'admin') return;
    // Always load both on mount
    admin.auditLogs({ limit: 100 }).then(setLogs).catch(function() {});
    admin.listUsers().then(setUsers).catch(function() {});
    setLoading(false);
  }, [user]);

  useEffect(function() {
    if (user?.role !== 'admin') return;
    if (tab === 'permissions' && users.length === 0) {
      admin.listUsers().then(setUsers).catch(function() {});
    }
  }, [tab]);

  const adminTabs = [
    { key: 'audit', label: 'Audit Logs' } as const,
    { key: 'users', label: 'Users' } as const,
    { key: 'permissions', label: 'Permissions' } as const,
  ];

  if (user?.role !== 'admin') {
    return (
      <div className="main-content">
        <div className="header"><div className="header-left"><span style={{ fontWeight: 600 }}>Admin</span></div></div>
        <div className="file-area" style={{ textAlign: 'center', paddingTop: 64, color: 'var(--text-secondary)' }}>
          Admin access only.
        </div>
      </div>
    );
  }

  return (
    <div className="main-content">
      <div className="header">
        <div className="header-left">
          <span style={{ fontWeight: 600, fontSize: 16 }}>Admin Panel</span>
        </div>
        <div className="header-right" style={{ display: 'flex', gap: 8 }}>
          {adminTabs.map(function(t) {
            return (
              <button key={t.key} className={'btn ' + (tab === t.key ? 'btn-primary' : '')} onClick={function() { setTab(t.key); }}>
                {t.label}
              </button>
            );
          })}
        </div>
      </div>
      <div className="file-area">
        {loading ? (
          <div className="empty-state"><div className="loading-spinner" /></div>
        ) : tab === 'audit' ? (
          <AuditLogView logs={logs} />
        ) : tab === 'users' ? (
          <UserListView users={users} />
        ) : (
          <PermissionsView users={users} />
        )}
      </div>
    </div>
  );
}

function AuditLogView({ logs }: { logs: AuditLogEntry[] }) {
  return (
    <table className="file-table">
      <thead>
        <tr>
          <th>Time</th>
          <th>User</th>
          <th>Action</th>
          <th>Detail</th>
          <th>IP</th>
        </tr>
      </thead>
      <tbody>
        {logs.map((l) => (
          <tr key={l.id} className="file-row">
            <td className="file-date">{new Date(l.created_at).toLocaleString('zh-CN')}</td>
            <td style={{ fontSize: 14 }}>{l.user_id}</td>
            <td style={{ fontSize: 14 }}>{l.action}</td>
            <td style={{ fontSize: 14, color: 'var(--text-secondary)' }}>{l.detail}</td>
            <td className="file-date">{l.ip}</td>
          </tr>
        ))}
        {logs.length === 0 && (
          <tr><td colSpan={5} style={{ textAlign: 'center', padding: 32, color: 'var(--text-tertiary)' }}>No audit logs</td></tr>
        )}
      </tbody>
    </table>
  );
}

function UserListView({ users }: { users: User[] }) {
  return (
    <table className="file-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Username</th>
          <th>Email</th>
          <th>Role</th>
          <th>Storage Used</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {users.map((u) => (
          <tr key={u.id} className="file-row">
            <td style={{ fontSize: 14 }}>{u.id}</td>
            <td style={{ fontSize: 14, fontWeight: 500 }}>{u.username}</td>
            <td style={{ fontSize: 14 }}>{u.email}</td>
            <td style={{ fontSize: 14 }}>{u.role}</td>
            <td className="file-date">{formatBytes(u.storage_used)} / {formatBytes(u.storage_quota)}</td>
            <td style={{ fontSize: 14 }}>
              <span style={{ color: u.is_active ? 'var(--success)' : 'var(--text-tertiary)' }}>
                {u.is_active ? 'Active' : 'Inactive'}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i];
}

function PermissionsView({ users }: { users: User[] }) {
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [perms, setPerms] = useState<Permission[]>([]);
  const [newPath, setNewPath] = useState('');
  const [newPerm, setNewPerm] = useState('read');
  const [loadingPerms, setLoadingPerms] = useState(false);

  useEffect(() => {
    if (!selectedUserId) { setPerms([]); return; }
    setLoadingPerms(true);
    admin.permissions.list(selectedUserId)
      .then(setPerms)
      .catch(() => setPerms([]))
      .finally(() => setLoadingPerms(false));
  }, [selectedUserId]);

  const handleAdd = async () => {
    if (!selectedUserId || !newPath.trim()) return;
    try {
      const p = await admin.permissions.create({ user_id: selectedUserId, file_path: newPath.trim(), perm: newPerm });
      setPerms(prev => [...prev, p]);
      setNewPath('');
    } catch (err: any) {
      alert(err.response?.data?.error || 'Failed to add permission');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await admin.permissions.delete(id);
      setPerms(prev => prev.filter(p => p.id !== id));
    } catch { /* ignore */ }
  };

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
        <label style={{ fontSize: 14 }}>User:</label>
        <select value={selectedUserId ?? ''} onChange={e => setSelectedUserId(e.target.value ? Number(e.target.value) : null)}
          style={{ padding: '6px 10px', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 14 }}>
          <option value="">Select user...</option>
          {users.map(u => <option key={u.id} value={u.id}>{u.username} ({u.id})</option>)}
        </select>
      </div>

      {selectedUserId && (
        <div style={{ marginBottom: 16, display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <input value={newPath} onChange={e => setNewPath(e.target.value)} placeholder="/path/to/folder"
            style={{ padding: '6px 10px', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 14, width: 240 }} />
          <select value={newPerm} onChange={e => setNewPerm(e.target.value)}
            style={{ padding: '6px 10px', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 14 }}>
            <option value="read">Read</option>
            <option value="write">Write</option>
            <option value="admin">Admin</option>
          </select>
          <button className="btn btn-primary" onClick={handleAdd}>Add</button>
        </div>
      )}

      {loadingPerms ? (
        <div style={{ padding: 32, textAlign: 'center' }}><div className="loading-spinner" /></div>
      ) : (
        <table className="file-table">
          <thead><tr><th>Path</th><th>Permission</th><th></th></tr></thead>
          <tbody>
            {perms.map(p => (
              <tr key={p.id} className="file-row">
                <td style={{ fontSize: 14, fontFamily: 'var(--font-mono)' }}>{p.file_path}</td>
                <td style={{ fontSize: 14 }}>{p.perm}</td>
                <td><button className="btn btn-danger" style={{ padding: '2px 8px', fontSize: 12 }} onClick={() => handleDelete(p.id)}>Remove</button></td>
              </tr>
            ))}
            {perms.length === 0 && <tr><td colSpan={3} style={{ textAlign: 'center', padding: 32, color: 'var(--text-tertiary)' }}>No permissions set</td></tr>}
          </tbody>
        </table>
      )}
    </div>
  );
}
