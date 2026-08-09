import { useEffect, useState } from 'react';
import type { FileItem } from '../api';
import { files } from '../api';

interface Props {
  file: FileItem;
  onClose: () => void;
}

export default function FilePreview({ file, onClose }: Props) {
  const [textContent, setTextContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const isImage = file.mime_type?.startsWith('image/');
  const isVideo = file.mime_type?.startsWith('video/');
  const isText = file.mime_type?.startsWith('text/') || file.mime_type === 'application/json';

  useEffect(() => {
    if (isText) {
      fetch(files.downloadBlobUrl(file.id))
        .then(r => r.text())
        .then(setTextContent)
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, [file.id]);

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
        {isImage ? (
          <img src={previewUrl} alt={file.name} style={{
            maxWidth: '90vw', maxHeight: '80vh', objectFit: 'contain',
            borderRadius: 8, boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
          }} />
        ) : isVideo ? (
          <video src={previewUrl} controls style={{
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
