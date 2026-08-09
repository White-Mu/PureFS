import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthStore, useUIStore } from '../store';

function fmtBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0) + ' ' + sizes[i];
}

export default function Sidebar() {
  const sidebarOpen = useUIStore((s) => s.sidebarOpen);
  const darkMode = useUIStore((s) => s.darkMode);
  const toggleDarkMode = useUIStore((s) => s.toggleDarkMode);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <aside className={`sidebar ${sidebarOpen ? '' : 'collapsed'}`}>
      <div className="sidebar-header">
        <span>PureFS</span>
      </div>
      <nav className="sidebar-nav">
        <NavLink to="/" end className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>📁 All Files</NavLink>
        <NavLink to="/favorites" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>⭐ Favorites</NavLink>
        <NavLink to="/pinned" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>📌 Pinned</NavLink>
        <NavLink to="/recent" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>🕐 Recent</NavLink>
        <NavLink to="/trash" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>🗑️ Trash</NavLink>
        <NavLink to="/shares" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>🔗 Shares</NavLink>
        <div style={{ flex: 1 }} />
        <div className="nav-item" onClick={toggleDarkMode} style={{ fontSize: 13, cursor: 'pointer' }}>
          {darkMode ? '☀️ Light Mode' : '🌙 Dark Mode'}
        </div>
        <div style={{ fontSize: 13, color: 'var(--text-secondary)', padding: '8px 12px' }}>
          {user?.username || 'User'}
          {user?.role === 'admin' ? <span style={{ color: 'var(--accent)', fontWeight: 500 }}> · Admin</span> : null}
        </div>
        {user && user.storage_quota > 0 && (
          <div style={{ fontSize: 11, color: 'var(--text-tertiary)', padding: '0 12px 8px' }}>
            {fmtBytes(user.storage_used)} / {fmtBytes(user.storage_quota)}
          </div>
        )}
        <NavLink to="/settings" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`} style={{ fontSize: 13 }}>
          ⚙ Settings
        </NavLink>
        {user?.role === 'admin' && (
          <NavLink to="/admin" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`} style={{ fontSize: 13 }}>
            🛡 Admin
          </NavLink>
        )}
        <div className="nav-item" onClick={handleLogout} style={{ fontSize: 13 }}>
          🚪 Sign Out
        </div>
      </nav>
    </aside>
  );
}
