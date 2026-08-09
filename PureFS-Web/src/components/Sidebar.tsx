import { NavLink, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuthStore, useUIStore } from '../store';

function fmtBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(i > 0 ? 1 : 0) + ' ' + sizes[i];
}

export default function Sidebar() {
  const { t, i18n } = useTranslation();
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

  const toggleLang = () => {
    const next = i18n.language === 'zh-CN' ? 'en-US' : 'zh-CN';
    i18n.changeLanguage(next);
  };

  return (
    <aside className={`sidebar ${sidebarOpen ? '' : 'collapsed'}`}>
      <div className="sidebar-header">
        <span>{t('app.name')}</span>
      </div>
      <nav className="sidebar-nav">
        <NavLink to="/" end className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          📁 {t('sidebar.allFiles')}
        </NavLink>
        <NavLink to="/favorites" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          ⭐ {t('sidebar.favorites')}
        </NavLink>
        <NavLink to="/pinned" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          📌 {t('sidebar.pinned')}
        </NavLink>
        <NavLink to="/recent" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          🕐 {t('sidebar.recent')}
        </NavLink>
        <NavLink to="/trash" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          🗑️ {t('sidebar.trash')}
        </NavLink>
        <NavLink to="/shares" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          🔗 {t('sidebar.shares')}
        </NavLink>
        <div style={{ flex: 1 }} />
        <div className="nav-item" onClick={toggleDarkMode} style={{ fontSize: 13, cursor: 'pointer' }}>
          {darkMode ? '☀️' : '🌙'} {t('settings.darkMode')}
        </div>
        <div className="nav-item" onClick={toggleLang} style={{ fontSize: 13, cursor: 'pointer' }}>
          🌐 {i18n.language === 'zh-CN' ? 'English' : '中文'}
        </div>
        <div style={{ fontSize: 13, color: 'var(--text-secondary)', padding: '8px 12px' }}>
          {user?.username || 'User'}
          {user?.role === 'admin' ? <span style={{ color: 'var(--accent)', fontWeight: 500 }}> · {t('sidebar.admin')}</span> : null}
        </div>
        {user && user.storage_quota > 0 && (
          <div style={{ fontSize: 11, color: 'var(--text-tertiary)', padding: '0 12px 8px' }}>
            {fmtBytes(user.storage_used)} / {fmtBytes(user.storage_quota)}
          </div>
        )}
        <NavLink to="/settings" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`} style={{ fontSize: 13 }}>
          ⚙ {t('sidebar.settings')}
        </NavLink>
        {user?.role === 'admin' && (
          <NavLink to="/admin" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`} style={{ fontSize: 13 }}>
            🛡 {t('sidebar.admin')}
          </NavLink>
        )}
        <div className="nav-item" onClick={handleLogout} style={{ fontSize: 13 }}>
          🚪 {t('sidebar.signOut')}
        </div>
      </nav>
    </aside>
  );
}
