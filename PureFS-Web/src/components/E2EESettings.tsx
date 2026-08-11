import { useState } from 'react';
import { auth } from '../api';
import {
  generateMasterKey, generateSalt, deriveKEK, wrapMasterKey,
} from '../utils/e2ee';
import { useE2EEStore } from '../store/e2ee';

// E2EESettings manages the End-to-End encryption master key lifecycle.
//
// Setup flow:
//   1. User enters a passphrase twice. We derive the KEK client-side.
//   2. A random 32-byte master key is generated, wrapped with the KEK, and the
//      wrapped key + salt are stored on the server.
//   3. The wrapped key is shown once so the user can back it up. A backup means
//      the master key can be recovered even if the passphrase is forgotten.
//
// Unlock flow (for existing users): enter the passphrase to unwrap the master
// key into memory. It is never persisted to localStorage or sent to the server.
export default function E2EESettings() {
  const enabled = useE2EEStore((s) => s.enabled);
  const unlock = useE2EEStore((s) => s.unlock);
  const lock = useE2EEStore((s) => s.lock);
  const setEnabled = useE2EEStore((s) => s.setEnabled);

  const [mode, setMode] = useState<'idle' | 'setup' | 'unlock'>('idle');
  const [passphrase, setPassphrase] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [info, setInfo] = useState('');
  const [backupKey, setBackupKey] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const resetForm = () => {
    setMode('idle');
    setPassphrase('');
    setConfirm('');
    setError('');
    setInfo('');
  };

  const handleSetup = async () => {
    setError('');
    setInfo('');
    if (passphrase.length < 8) {
      setError('Passphrase must be at least 8 characters.');
      return;
    }
    if (passphrase !== confirm) {
      setError('Passphrases do not match.');
      return;
    }
    setBusy(true);
    try {
      const salt = generateSalt();
      const masterKey = generateMasterKey();
      const kek = await deriveKEK(passphrase, salt);
      const wrappedKey = await wrapMasterKey(masterKey, kek);

      await auth.e2eeSetup({ salt, wrapped_key: wrappedKey });
      setEnabled(true);
      setBackupKey(wrappedKey);
      setInfo('End-to-end encryption is now enabled.');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to enable encryption.');
    } finally {
      setBusy(false);
    }
  };

  const handleUnlock = async () => {
    setError('');
    setInfo('');
    if (!passphrase) {
      setError('Enter your passphrase.');
      return;
    }
    setBusy(true);
    try {
      const ok = await unlock(passphrase);
      if (!ok) {
        setError('Incorrect passphrase. The master key could not be unlocked.');
      } else {
        setInfo('Master key unlocked for this session.');
        setMode('idle');
        setPassphrase('');
      }
    } finally {
      setBusy(false);
    }
  };

  const handleDisable = async () => {
    if (!window.confirm(
      'Disabling End-to-End encryption will make existing encrypted files permanently undecryptable. Are you sure?',
    )) return;
    setBusy(true);
    setError('');
    try {
      await auth.e2eeDisable();
      lock();
      setEnabled(false);
      setBackupKey(null);
      resetForm();
      setInfo('End-to-End encryption disabled. Existing encrypted files cannot be decrypted anymore.');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to disable encryption.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{
      background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
      borderRadius: 'var(--radius-md)', padding: 24, marginBottom: 16,
    }}>
      <h3 style={{ marginBottom: 16 }}>🔐 End-to-End Encryption</h3>

      {error && (
        <div className="toast toast-error" style={{ marginBottom: 16 }}>{error}</div>
      )}
      {info && (
        <div style={{
          background: 'var(--success)', color: '#fff', padding: '10px 14px',
          borderRadius: 'var(--radius-sm)', marginBottom: 16, fontSize: 14,
        }}>{info}</div>
      )}

      {enabled ? (
        <div>
          <p style={{ color: 'var(--text-secondary)', fontSize: 14, marginBottom: 12 }}>
            End-to-end encryption is <strong style={{ color: 'var(--success)' }}>enabled</strong>.
            Files are encrypted in your browser before upload; the server only stores ciphertext.
          </p>
          {backupKey && (
            <div style={{ marginBottom: 16 }}>
              <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 4 }}>
                ⚠️ Save this backup key somewhere safe. It can unlock your files even if you forget your passphrase.
              </p>
              <code style={{
                padding: '8px 12px', background: 'var(--bg-tertiary)', borderRadius: 'var(--radius-sm)',
                display: 'block', fontSize: 12, wordBreak: 'break-all',
              }}>{backupKey}</code>
            </div>
          )}
          <button className="btn btn-danger" onClick={handleDisable} disabled={busy}>
            {busy ? 'Disabling...' : 'Disable E2EE'}
          </button>
        </div>
      ) : mode === 'idle' ? (
        <div>
          <p style={{ color: 'var(--text-secondary)', fontSize: 14, marginBottom: 12 }}>
            Encrypt files in your browser before they leave your device. The server never sees
            your content or your key. Forgetting your passphrase makes data unrecoverable —
            keep a backup key.
          </p>
          <button className="btn btn-primary" onClick={() => setMode('setup')}>Set up E2EE</button>
          <button
            className="btn"
            style={{ marginLeft: 8 }}
            onClick={() => { setMode('unlock'); setError(''); setInfo(''); }}
          >
            Unlock master key
          </button>
        </div>
      ) : mode === 'setup' ? (
        <div>
          <p style={{ fontSize: 14, marginBottom: 12 }}>
            Choose a passphrase to protect your master key. This passphrase is never sent to the server.
          </p>
          <div className="form-group">
            <label>Passphrase</label>
            <input
              type="password" value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              placeholder="At least 8 characters"
              style={{ width: '100%', maxWidth: 360 }}
            />
          </div>
          <div className="form-group">
            <label>Confirm passphrase</label>
            <input
              type="password" value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="Repeat passphrase"
              style={{ width: '100%', maxWidth: 360 }}
            />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-primary" onClick={handleSetup} disabled={busy}>
              {busy ? 'Encrypting...' : 'Enable E2EE'}
            </button>
            <button className="btn" onClick={resetForm} disabled={busy}>Cancel</button>
          </div>
        </div>
      ) : (
        <div>
          <p style={{ fontSize: 14, marginBottom: 12 }}>
            Enter your passphrase to unlock the master key for this session.
          </p>
          <div className="form-group">
            <label>Passphrase</label>
            <input
              type="password" value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleUnlock(); }}
              placeholder="Your E2EE passphrase"
              style={{ width: '100%', maxWidth: 360 }}
            />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-primary" onClick={handleUnlock} disabled={busy}>
              {busy ? 'Unlocking...' : 'Unlock'}
            </button>
            <button className="btn" onClick={resetForm} disabled={busy}>Cancel</button>
          </div>
        </div>
      )}
    </div>
  );
}
