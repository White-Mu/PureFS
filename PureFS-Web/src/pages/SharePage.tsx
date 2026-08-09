import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import api from '../api/client';

const API_BASE = import.meta.env.VITE_API_BASE || '';

interface ShareDetail {
  file_name: string;
  file_size: number;
  file_type: string;
  mime_type: string;
  can_download: boolean;
  expires_at: string | null;
  access_count: number;
  max_accesses: number;
  is_active: boolean;
}

export default function SharePage() {
  const { token } = useParams<{ token: string }>();
  const [share, setShare] = useState<ShareDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [textContent, setTextContent] = useState<string | null>(null);

  const isTextType = (mime?: string) => mime?.startsWith('text/') || mime === 'application/json';
  const contentUrl = token ? `${API_BASE}/api/public/shares/${token}/content` : '';

  useEffect(() => {
    if (!token) return;
    api.get<ShareDetail>(`/public/shares/${token}`)
      .then(r => {
        setShare(r.data);
        // If it's a text file, fetch the content
        if (isTextType(r.data.mime_type)) {
          return fetch(contentUrl).then(res => res.text()).then(setTextContent);
        }
      })
      .catch(err => setError(err.response?.data?.error || '分享不存在或已失效'))
      .finally(() => setLoading(false));
  }, [token]);

  const formatSize = (bytes: number) => {
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  };

  const isImage = () => share?.mime_type?.startsWith('image/');
  const isVideo = () => share?.mime_type?.startsWith('video/');

  if (loading) {
    return (
      <div className="auth-container">
        <div className="loading-spinner" style={{ width: 32, height: 32 }} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="auth-container" style={{ flexDirection: 'column', gap: 16 }}>
        <div style={{ fontSize: 48 }}>🔗</div>
        <h2 style={{ color: 'var(--text-secondary)', fontWeight: 500 }}>{error}</h2>
      </div>
    );
  }

  if (!share || !token) return null;

  return (
    <div style={{
      minHeight: '100vh',
      background: 'var(--bg-primary)',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
    }}>
      {/* Header bar */}
      <div style={{
        width: '100%',
        borderBottom: '1px solid var(--border-color)',
        padding: '12px 24px',
        background: 'var(--bg-secondary)',
        display: 'flex',
        alignItems: 'center',
        gap: 12,
      }}>
        <span style={{ fontWeight: 600, fontSize: 15 }}>PureFS</span>
        <span style={{ color: 'var(--text-tertiary)', fontSize: 13, flex: 1, textAlign: 'right' }}>
          {formatSize(share.file_size)}
        </span>
      </div>

      {/* Content area */}
      <div style={{
        width: '100%',
        maxWidth: 900,
        padding: '24px 16px',
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
      }}>
        {isImage() ? (
          <img
            src={contentUrl}
            alt={share.file_name}
            style={{
              maxWidth: '100%',
              maxHeight: '80vh',
              objectFit: 'contain',
              borderRadius: 'var(--radius-md)',
              boxShadow: 'var(--shadow-md)',
            }}
          />
        ) : isVideo() ? (
          <video
            src={contentUrl}
            controls
            style={{
              maxWidth: '100%',
              maxHeight: '80vh',
              borderRadius: 'var(--radius-md)',
            }}
          >
            您的浏览器不支持视频播放
          </video>
        ) : textContent !== null ? (
          <pre style={{
            width: '100%',
            padding: 16,
            background: 'var(--bg-secondary)',
            border: '1px solid var(--border-color)',
            borderRadius: 'var(--radius-md)',
            fontSize: 14,
            lineHeight: 1.6,
            overflow: 'auto',
            maxHeight: '80vh',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            margin: 0,
          }}>{textContent}</pre>
        ) : (
          <div style={{
            textAlign: 'center',
            padding: '64px 32px',
            color: 'var(--text-tertiary)',
          }}>
            <div style={{ fontSize: 56, marginBottom: 16 }}>
              {share.file_type === 'directory' ? '📁' : '📄'}
            </div>
            <h2 style={{ color: 'var(--text-secondary)', fontWeight: 500, marginBottom: 8 }}>
              {share.file_name}
            </h2>
            <p style={{ fontSize: 14, marginBottom: 24 }}>
              此文件无法在线预览
            </p>
            {share.can_download && (
              <a
                href={`${contentUrl}?download=1`}
                className="btn btn-primary"
                style={{ textDecoration: 'none', padding: '10px 32px' }}
              >
                ⬇ 下载文件
              </a>
            )}
          </div>
        )}

        {/* Download button for viewable types */}
        {share.can_download && (isImage() || isVideo() || isTextType(share.mime_type)) && (
          <a
            href={`${contentUrl}?download=1`}
            className="btn"
            style={{
              marginTop: 16,
              textDecoration: 'none',
              padding: '8px 24px',
            }}
          >
            ⬇ 下载
          </a>
        )}
      </div>
    </div>
  );
}
