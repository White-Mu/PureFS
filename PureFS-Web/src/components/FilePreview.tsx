import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { FileItem } from '../api';
import { files } from '../api';
import { useE2EEStore } from '../store/e2ee';
import { decryptFile } from '../utils/e2ee';

interface Props {
  file: FileItem;
  onClose: () => void;
}

export default function FilePreview({ file, onClose }: Props) {
  const { t } = useTranslation();
  const [textContent, setTextContent] = useState<string | null>(null);
  const [decryptedUrl, setDecryptedUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const e2eeMasterKey = useE2EEStore((s) => s.masterKey);

  const isImage = file.mime_type?.startsWith('image/');
  const isVideo = file.mime_type?.startsWith('video/');
  const isText = file.mime_type?.startsWith('text/') || file.mime_type === 'application/json';

  const isEncrypted = !!file.is_e2ee;

  useEffect(() => {
    const previewUrl = `${import.meta.env.VITE_API_BASE || ''}/api/files/${file.id}/preview?token=${localStorage.getItem('token')}`;

    // E2EE files are ciphertext on the server. Decrypt locally for preview.
    if (isEncrypted) {
      if (!e2eeMasterKey || !file.dek_ciphertext) {
        setError(t('e2ee.fileLocked'));
        setLoading(false);
        return;
      }
      files.downloadBlob(file.id)
        .then((blob) => blob.arrayBuffer())
        .then((buf) => decryptFile(buf, file.dek_ciphertext!, e2eeMasterKey!))
        .then((plaintext) => {
          const blob = new Blob([plaintext], { type: file.mime_type || 'application/octet-stream' });
          if (isText) {
            return blob.text().then((text) => { setTextContent(text); });
          }
          setDecryptedUrl(URL.createObjectURL(blob));
        })
        .catch(() => setError(t('e2ee.decryptFailed')))
        .finally(() => setLoading(false));
      return;
    }

    if (isText) {
      fetch(previewUrl)
        .then(r => r.text())
        .then(setTextContent)
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, [file.id, isEncrypted, isText, t]);

  const previewUrl = `${import.meta.env.VITE_API_BASE || ''}/api/files/${file.id}/preview?token=${localStorage.getItem('token')}`;

  return (
    <div className="modal-overlay" onClick={onClose} style={{ background: 'rgba(0,0,0,0.7)', zIndex: 1001 }}>
      <div onClick={(e) => e.stopPropagation()} style={{
        position: 'relative',
        maxWidth: '90vw',
        maxHeight: '90vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
      }}>
        {/* Title bar */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 12, marginBottom: 8,
          color: '#fff', fontSize: 14,
        }}>
          <span style={{ fontWeight: 500 }}>{file.name}</span>
          <button onClick={onClose} style={{
            background: 'rgba(255,255,255,0.15)', border: 'none', color: '#fff',
            padding: '4px 10px', borderRadius: 4, cursor: 'pointer', fontSize: 16,
          }}>✕</button>
        </div>

        {/* Content */}
        {error ? (
          <div style={{
            background: 'rgba(255,0,0,0.15)', border: '1px solid rgba(255,0,0,0.4)',
            color: '#fff', padding: '16px 24px', borderRadius: 8, fontSize: 14,
          }}>{error}</div>
        ) : isImage ? (
          <img src={isEncrypted ? decryptedUrl || undefined : previewUrl} alt={file.name} style={{
            maxWidth: '90vw', maxHeight: '80vh', objectFit: 'contain',
            borderRadius: 8, boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
          }} />
        ) : isVideo ? (
          <video src={isEncrypted ? decryptedUrl || undefined : previewUrl} controls style={{
            maxWidth: '90vw', maxHeight: '80vh', borderRadius: 8,
            boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
          }} />
        ) : isText ? (
          <pre style={{
            background: '#1e1e1e', color: '#d4d4d4', padding: 20, borderRadius: 8,
            maxWidth: '90vw', maxHeight: '80vh', overflow: 'auto', fontSize: 13,
            lineHeight: 1.5, fontFamily: 'var(--font-mono)',
          }}>
            {loading ? 'Loading...' : textContent}
          </pre>
        ) : null}
      </div>
    </div>
  );
}
