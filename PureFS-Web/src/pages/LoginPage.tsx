import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '../store';
import { auth } from '../api';

export default function LoginPage() {
  const { t } = useTranslation();
  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [email, setEmail] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [totpRequired, setTotpRequired] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username, password, totpCode || undefined);
      navigate('/');
    } catch (err: any) {
      if (err.message === 'TOTP_REQUIRED') {
        setTotpRequired(true);
      } else {
        setError(err.response?.data?.error || t('login.loginFailed'));
      }
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) { setError(t('login.emailRequired')); return; }
    setLoading(true);
    setError('');
    try {
      await auth.register({ username, email, password });
      // Auto-login after register
      await login(username, password);
      navigate('/');
    } catch (err: any) {
      setError(err.response?.data?.error || t('login.registrationFailed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-container" style={{ flexDirection: 'column', gap: 16 }}>
      <form className="auth-card" onSubmit={mode === 'login' ? handleLogin : handleRegister}>
        <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
          <button type="button"
            className={`btn ${mode === 'login' ? 'btn-primary' : ''}`}
            style={{ flex: 1, justifyContent: 'center' }}
            onClick={() => { setMode('login'); setError(''); setTotpRequired(false); }}>
            {t('login.signIn')}
          </button>
          <button type="button"
            className={`btn ${mode === 'register' ? 'btn-primary' : ''}`}
            style={{ flex: 1, justifyContent: 'center' }}
            onClick={() => { setMode('register'); setError(''); setTotpRequired(false); }}>
            {t('login.register')}
          </button>
        </div>

        <h1 style={{ marginTop: 8 }}>{t('login.title')}</h1>
        <p>{mode === 'login' ? t('login.signInSubtitle') : t('login.registerSubtitle')}</p>

        {error && <div className="toast toast-error" style={{ marginBottom: 16 }}>{error}</div>}

        <div className="form-group">
          <label>{t('login.username')}</label>
          <input type="text" value={username} onChange={(e) => setUsername(e.target.value)}
            disabled={loading} autoFocus required />
        </div>

        {mode === 'register' && (
          <div className="form-group">
            <label>{t('login.email')}</label>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)}
              disabled={loading} required />
          </div>
        )}

        <div className="form-group">
          <label>{t('login.password')}</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}
            disabled={loading} required />
        </div>

        {totpRequired && (
          <div className="form-group">
            <label>{t('login.totpCode')}</label>
            <input type="text" value={totpCode} onChange={(e) => setTotpCode(e.target.value)}
              placeholder={t('login.totpPlaceholder')} disabled={loading} autoFocus required />
          </div>
        )}

        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={loading} style={{ flex: 1, justifyContent: 'center' }}>
            {loading ? <span className="loading-spinner" /> : mode === 'login' ? t('login.signIn') : t('login.register')}
          </button>
        </div>
      </form>
    </div>
  );
}
