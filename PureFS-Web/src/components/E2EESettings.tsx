import { useState } from 'react';
import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation();
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
      setError(t('e2ee.passphraseShort'));
      return;
    }
    if (passphrase !== confirm) {
      setError(t('e2ee.passphraseMismatch'));
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
      setInfo(t('e2ee.enabledSuccess'));
    } catch (err: any) {
      setError(err.response?.data?.error || t('e2ee.setupFailed'));
    } finally {
      setBusy(false);
    }
  };

  const handleUnlock = async () => {
    setError('');
    setInfo('');
    if (!passphrase) {
      setError(t('e2ee.passphrase'));
      return;
    }
    setBusy(true);
    try {
      const ok = await unlock(passphrase);
      if (!ok) {
        setError(t('e2ee.unlockWrong'));
      } else {
        setInfo(t('e2ee.unlockSuccess'));
        setMode('idle');
        setPassphrase('');
      }
    } finally {
      setBusy(false);
    }
  };

  const handleDisable = async () => {
    if (!window.confirm(t('e2ee.disableConfirm'))) return;
    setBusy(true);
    setError('');
    try {
      await auth.e2eeDisable();
      lock();
      setEnabled(false);
      setBackupKey(null);
      resetForm();
      setInfo(t('e2ee.disabledNotice'));
    } catch (err: any) {
      setError(err.response?.data?.error || t('e2ee.disableFailed'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{
      background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
      borderRadius: 'var(--radius-md)', padding: 24, marginBottom: 16,
    }}>
      <h3 style={{ marginBottom: 16 }}>🔐 {t('e2ee.title')}</h3>

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
            {t('e2ee.title')} <strong style={{ color: 'var(--success)' }}>{t('e2ee.enabled')}</strong>.
            {t('e2ee.enabledDesc')}
          </p>
          {backupKey && (
            <div style={{ marginBottom: 16 }}>
              <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 4 }}>
                ⚠️ {t('e2ee.backupKeyWarning')}
              </p>
              <code style={{
                padding: '8px 12px', background: 'var(--bg-tertiary)', borderRadius: 'var(--radius-sm)',
                display: 'block', fontSize: 12, wordBreak: 'break-all',
              }}>{backupKey}</code>
            </div>
          )}
          <button className="btn btn-danger" onClick={handleDisable} disabled={busy}>
            {busy ? t('e2ee.disabling') : t('e2ee.disable')}
          </button>
        </div>
      ) : mode === 'idle' ? (
        <div>
          <p style={{ color: 'var(--text-secondary)', fontSize: 14, marginBottom: 12 }}>
            {t('e2ee.setupIntro')}
          </p>
          <button className="btn btn-primary" onClick={() => setMode('setup')}>{t('e2ee.setup')}</button>
          <button
            className="btn"
            style={{ marginLeft: 8 }}
            onClick={() => { setMode('unlock'); setError(''); setInfo(''); }}
          >
            {t('e2ee.unlockMasterKey')}
          </button>
        </div>
      ) : mode === 'setup' ? (
        <div>
          <p style={{ fontSize: 14, marginBottom: 12 }}>
            {t('e2ee.setupPrompt')}
          </p>
          <div className="form-group">
            <label>{t('e2ee.passphrase')}</label>
            <input
              type="password" value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              placeholder={t('e2ee.passphrasePlaceholder')}
              style={{ width: '100%', maxWidth: 360 }}
            />
          </div>
          <div className="form-group">
            <label>{t('e2ee.confirmPassphrase')}</label>
            <input
              type="password" value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder={t('e2ee.confirmPlaceholder')}
              style={{ width: '100%', maxWidth: 360 }}
            />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-primary" onClick={handleSetup} disabled={busy}>
              {busy ? t('e2ee.encrypting') : t('e2ee.enable')}
            </button>
            <button className="btn" onClick={resetForm} disabled={busy}>{t('e2ee.cancel')}</button>
          </div>
        </div>
      ) : (
        <div>
          <p style={{ fontSize: 14, marginBottom: 12 }}>
            {t('e2ee.unlockPrompt')}
          </p>
          <div className="form-group">
            <label>{t('e2ee.passphrase')}</label>
            <input
              type="password" value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleUnlock(); }}
              placeholder={t('e2ee.passphrase')}
              style={{ width: '100%', maxWidth: 360 }}
            />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-primary" onClick={handleUnlock} disabled={busy}>
              {busy ? t('e2ee.unlocking') : t('e2ee.unlock')}
            </button>
            <button className="btn" onClick={resetForm} disabled={busy}>{t('e2ee.cancel')}</button>
          </div>
        </div>
      )}
    </div>
  );
}
