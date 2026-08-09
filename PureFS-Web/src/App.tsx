import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { useAuthStore, useUIStore } from './store';
import LoginPage from './pages/LoginPage';
import FilesPage from './pages/FilesPage';
import SharePage from './pages/SharePage';
import SettingsPage from './pages/SettingsPage';
import SharesPage from './pages/SharesPage';
import AdminPage from './pages/AdminPage';
import TrashPage from './pages/TrashPage';
import Sidebar from './components/Sidebar';
import './styles/globals.css';
import './styles/components.css';

function AppContent() {
  const token = useAuthStore((s) => s.token);
  const darkMode = useUIStore((s) => s.darkMode);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', darkMode ? 'dark' : 'light');
  }, [darkMode]);

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/share/:token" element={<SharePage />} />
      {token ? (
        <Route element={
          <div className="app-layout" style={{ display: 'flex', height: '100vh' }}>
            <Sidebar />
            <Outlet />
          </div>
        }>
          <Route path="/" element={<FilesPage />} />
          <Route path="/favorites" element={<FilesPage viewFilter="favorites" />} />
          <Route path="/pinned" element={<FilesPage viewFilter="pinned" />} />
          <Route path="/recent" element={<FilesPage viewFilter="recent" />} />
          <Route path="/trash" element={<TrashPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/shares" element={<SharesPage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      ) : (
        <Route path="*" element={<Navigate to="/login" replace />} />
      )}
    </Routes>
  );
}

function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}

export default App;
