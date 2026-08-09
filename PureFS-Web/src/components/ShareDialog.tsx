import { useState } from 'react';
import type { FileItem } from '../api';
import { shares } from '../api';

interface ShareDialogProps {
  file: FileItem;
  onClose: () => void;
}

export default function ShareDialog({ file, onClose }: ShareDialogProps) {
  const [password, setPassword] = useState('');
  const [expiresIn, setExpiresIn] = useState('');
  const [maxAccesses, setMaxAccesses] = useState('');
  const [shareLink, setShareLink] = useState('');
  const [error, setError] = useState('');

  const handleCreate = async () => {
    try {
      const share = await shares.create({
        file_id: file.id,
        password: password || undefined,
        expires_in: expiresIn || undefined,
        max_accesses: maxAccesses ? parseInt(maxAccesses) : undefined,
        can_download: true,
      });
      setShareLink(`${window.location.origin}/share/${share.token}`);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create share');
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span>Share: {file.name}</span>
          <button className="btn-icon" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          {error && <div className="toast toast-error" style={{ marginBottom: 16 }}>{error}</div>}

          {shareLink ? (
            <div>
              <p style={{ marginBottom: 12, fontSize: 14, color: 'var(--text-secondary)' }}>Share link created!</p>
              <div style={{ display: 'flex', gap: 8 }}>
                <input
                  value={shareLink}
                  readOnly
                  style={{ flex: 1, padding: '8px 12px', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', background: 'var(--bg-primary)', color: 'var(--text-primary)' }}
                  onClick={(e) => (e.target as HTMLInputElement).select()}
                />
                <button className="btn btn-primary" onClick={() => navigator.clipboard.writeText(shareLink)}>Copy</button>
              </div>
            </div>
          ) : (
            <>
              <div className="form-group">
                <label>Password (optional)</label>
                <input type="text" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Leave empty for no password" />
              </div>
              <div className="form-group">
                <label>Expires in (optional)</label>
                <input type="text" value={expiresIn} onChange={(e) => setExpiresIn(e.target.value)} placeholder="e.g., 24h, 7d, 30d" />
              </div>
              <div className="form-group">
                <label>Max accesses (optional)</label>
                <input type="number" value={maxAccesses} onChange={(e) => setMaxAccesses(e.target.value)} placeholder="Unlimited" />
              </div>
              <div className="form-actions">
                <button className="btn btn-primary" onClick={handleCreate}>Create Share Link</button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
