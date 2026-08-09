import { useState, useEffect } from 'react';
import { useAuthStore } from '../store';
import { admin } from '../api';
import api from '../api/client';
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
    admin.auditLogs({ limit: 100 }).then((logs) => setLogs(logs || [])).catch(function() {});
    admin.listUsers().then((users) => setUsers(users || [])).catch(function() {});
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
  const [showCreate, setShowCreate] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', email: '', password: '', role: 'user' });
  const [error, setError] = useState('');

  const handleCreateUser = async () => {
    if (!newUser.username || !newUser.password) { setError('Username and password required'); return; }
    try {
      await api.post('/admin/users', newUser);
      setShowCreate(false);
      setNewUser({ username: '', email: '', password: '', role: 'user' });
      window.location.reload();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create user');
    }
  };

  const handleToggleUser = async (id: number, active: boolean) => {
    try {
      await api.patch(`/admin/users/${id}/toggle`, { active: !active });
      window.location.reload();
    } catch (err: any) {
      alert(err.response?.data?.error || 'Failed');
    }
  };

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span style={{ fontSize: 14, color: 'var(--text-secondary)' }}>{users.length} users</span>
        <button className="btn btn-primary" style={{ fontSize: 13 }} onClick={() => setShowCreate(!showCreate)}>
          + New User
        </button>
      </div>

      {showCreate && (
        <div style={{
          background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)', padding: 16, marginBottom: 16,
          display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'flex-end',
        }}>
          <div><label style={{ fontSize: 12, display: 'block' }}>Username</label>
            <input value={newUser.username} onChange={e => setNewUser(p => ({ ...p, username: e.target.value }))}
              style={{ padding: '4px 8px', width: 120, border: '1px solid var(--border-color)', borderRadius: 4, background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13 }} />
          </div>
          <div><label style={{ fontSize: 12, display: 'block' }}>Email</label>
            <input value={newUser.email} onChange={e => setNewUser(p => ({ ...p, email: e.target.value }))}
              style={{ padding: '4px 8px', width: 160, border: '1px solid var(--border-color)', borderRadius: 4, background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13 }} />
          </div>
          <div><label style={{ fontSize: 12, display: 'block' }}>Password</label>
            <input type="password" value={newUser.password} onChange={e => setNewUser(p => ({ ...p, password: e.target.value }))}
              style={{ padding: '4px 8px', width: 120, border: '1px solid var(--border-color)', borderRadius: 4, background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13 }} />
          </div>
          <div><label style={{ fontSize: 12, display: 'block' }}>Role</label>
            <select value={newUser.role} onChange={e => setNewUser(p => ({ ...p, role: e.target.value }))}
              style={{ padding: '4px 8px', border: '1px solid var(--border-color)', borderRadius: 4, background: 'var(--bg-primary)', color: 'var(--text-primary)', fontSize: 13 }}>
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <button className="btn btn-primary" style={{ fontSize: 13, padding: '4px 12px' }} onClick={handleCreateUser}>Create</button>
          <button className="btn" style={{ fontSize: 13, padding: '4px 12px' }} onClick={() => setShowCreate(false)}>Cancel</button>
          {error && <span style={{ color: 'var(--danger)', fontSize: 12 }}>{error}</span>}
        </div>
      )}

      <table className="file-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Username</th>
            <th>Email</th>
            <th>Role</th>
            <th>Storage Used</th>
            <th>Status</th>
            <th></th>
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
              <td>
                <button className="btn" style={{ fontSize: 11, padding: '2px 6px' }}
                  onClick={() => handleToggleUser(u.id, u.is_active)}>
                  {u.is_active ? 'Disable' : 'Enable'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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
