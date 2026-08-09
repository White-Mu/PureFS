import { useState, useRef, useEffect } from 'react';
import { useAuthStore } from '../store';
import { auth } from '../api';
import QRCode from 'qrcode';

export default function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  const [step, setStep] = useState<'idle' | 'setup' | 'verify' | 'done'>('idle');
  const [secret, setSecret] = useState('');
  const [uri, setUri] = useState('');
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const qrCanvasRef = useRef<HTMLCanvasElement>(null);

  const handleSetup = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await auth.setupTOTP();
      setSecret(res.secret);
      setUri(res.uri);
      setStep('setup');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to setup TOTP');
    } finally {
      setLoading(false);
    }
  };

  // Generate QR code locally when URI is set
  useEffect(() => {
    if (uri && qrCanvasRef.current) {
      QRCode.toCanvas(qrCanvasRef.current, uri, { width: 200, margin: 1 });
    }
  }, [uri]);

  const handleEnable = async () => {
    setLoading(true);
    setError('');
    try {
      await auth.enableTOTP(code);
      setStep('done');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Invalid code');
    } finally {
      setLoading(false);
    }
  };

  const handleDisable = async () => {
    if (!confirm('Disable two-factor authentication?')) return;
    setLoading(true);
    setError('');
    try {
      await auth.disableTOTP();
      setStep('idle');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to disable');
    } finally {
      setLoading(false);
    }
  };

  if (!user) return null;

  return (
    <div className="main-content">
      <div className="header">
        <div className="header-left">
          <span style={{ fontWeight: 600, fontSize: 16 }}>Settings</span>
        </div>
      </div>
      <div className="file-area" style={{ maxWidth: 600 }}>
        <div style={{
          background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)', padding: 24, marginBottom: 16,
        }}>
          <h3 style={{ marginBottom: 16 }}>Two-Factor Authentication (TOTP)</h3>

          {error && (
            <div className="toast toast-error" style={{ marginBottom: 16 }}>{error}</div>
          )}

          {user.totp_enabled ? (
            <div>
              <p style={{ color: 'var(--text-secondary)', fontSize: 14, marginBottom: 12 }}>
                Two-factor authentication is currently <strong style={{ color: 'var(--success)' }}>enabled</strong>.
              </p>
              <button className="btn btn-danger" onClick={handleDisable} disabled={loading}>
                {loading ? 'Disabling...' : 'Disable 2FA'}
              </button>
            </div>
          ) : step === 'idle' ? (
            <div>
              <p style={{ color: 'var(--text-secondary)', fontSize: 14, marginBottom: 12 }}>
                Add an extra layer of security to your account. Use an authenticator app like Google Authenticator or Authy.
              </p>
              <button className="btn btn-primary" onClick={handleSetup} disabled={loading}>
                {loading ? 'Setting up...' : 'Set up 2FA'}
              </button>
            </div>
          ) : step === 'setup' ? (
            <div>
              <p style={{ fontSize: 14, marginBottom: 12 }}>
                Scan this QR code with your authenticator app, or enter the secret key manually:
              </p>
              <div style={{
                background: '#fff', padding: 16, borderRadius: 8, marginBottom: 16,
                display: 'flex', justifyContent: 'center',
              }}>
                <canvas ref={qrCanvasRef} width={200} height={200} style={{ width: 200, height: 200 }} />
              </div>
              <div style={{ marginBottom: 16 }}>
                <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 4 }}>Secret Key</label>
                <code style={{
                  padding: '8px 12px', background: 'var(--bg-tertiary)', borderRadius: 'var(--radius-sm)',
                  display: 'block', fontSize: 14, wordBreak: 'break-all',
                }}>{secret}</code>
              </div>
              <div className="form-group">
                <label>Verify code</label>
                <input type="text" placeholder="Enter 6-digit code" value={code}
                  onChange={(e) => setCode(e.target.value)}
                  maxLength={6} style={{ width: 200 }}
                />
              </div>
              <button className="btn btn-primary" onClick={handleEnable} disabled={loading || code.length !== 6}>
                {loading ? 'Verifying...' : 'Verify & Enable'}
              </button>
            </div>
          ) : (
            <div>
              <p style={{ color: 'var(--success)', fontSize: 14, fontWeight: 500 }}>
                ✓ Two-factor authentication is now enabled.
              </p>
            </div>
          )}
        </div>

        {/* Account info */}
        <div style={{
          background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)', padding: 24,
        }}>
          <h3 style={{ marginBottom: 16 }}>Account</h3>
          <div style={{ fontSize: 14, display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div><span style={{ color: 'var(--text-secondary)' }}>Username:</span> {user.username}</div>
            <div><span style={{ color: 'var(--text-secondary)' }}>Email:</span> {user.email}</div>
            <div><span style={{ color: 'var(--text-secondary)' }}>Role:</span> {user.role}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
