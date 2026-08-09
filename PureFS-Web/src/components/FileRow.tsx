import type { FileItem } from '../api';

interface FileRowProps {
  file: FileItem;
  selected: boolean;
  onSelect: (multi: boolean) => void;
  onDoubleClick: () => void;
  renaming: boolean;
  renameValue?: string;
  onRenameChange?: (v: string) => void;
  onRenameSubmit?: () => void;
  onContextMenu: (e: React.MouseEvent) => void;
}

export default function FileRow({
  file, selected, onSelect, onDoubleClick,
  renaming, renameValue, onRenameChange, onRenameSubmit,
  onContextMenu,
}: FileRowProps) {
  const formatSize = (bytes: number) => {
    if (bytes === 0) return '-';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
  };

  const formatDate = (date: string) => {
    const d = new Date(date);
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const icon = file.file_type === 'directory' ? '📁' :
    file.mime_type?.startsWith('image/') ? '🖼️' :
    file.mime_type?.startsWith('video/') ? '🎬' :
    file.mime_type?.startsWith('audio/') ? '🎵' :
    file.mime_type?.includes('pdf') ? '📕' :
    file.mime_type?.includes('zip') || file.mime_type?.includes('rar') || file.mime_type?.includes('tar') ? '📦' : '📄';

  return (
    <tr
      className={`file-row ${selected ? 'selected' : ''}`}
      onClick={(e) => { if (renaming) return; onSelect(e.ctrlKey || e.metaKey); }}
      onDoubleClick={(e) => { if (renaming) return; onDoubleClick(e); }}
      onContextMenu={(e) => { e.preventDefault(); onContextMenu(e); }}
    >
      <td><span className="file-icon">{icon}</span></td>
      <td>
        <div className="file-name-cell">
          <span className="file-name">
            {renaming ? (
              <span style={{ display: 'inline-flex', gap: 4, alignItems: 'center' }}>
                <input
                  value={renameValue}
                  onChange={(e) => onRenameChange?.(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') onRenameSubmit?.(); if (e.key === 'Escape') { e.stopPropagation(); onRenameSubmit?.(); } }}
                  onClick={(e) => e.stopPropagation()}
                  autoFocus
                  style={{ padding: '2px 6px', fontSize: 14, border: '1px solid var(--accent)', borderRadius: 4, background: 'var(--bg-primary)', color: 'var(--text-primary)', width: 160 }}
                />
                <button type="button" onClick={(e) => { e.stopPropagation(); onRenameSubmit?.(); }} style={{ padding: '0 8px', fontSize: 14, cursor: 'pointer', border: '1px solid var(--accent)', borderRadius: 4, background: 'var(--accent)', color: 'var(--accent-text)' }}>✓</button>
                <button type="button" onClick={(e) => { e.stopPropagation(); onRenameSubmit?.(); }} style={{ padding: '0 8px', fontSize: 14, cursor: 'pointer', border: '1px solid var(--border-color)', borderRadius: 4, background: 'var(--bg-tertiary)', color: 'var(--text-secondary)' }}>✕</button>
              </span>
            ) : (
              file.name
            )}
          </span>
          {file.is_pinned && <span className="pinned-badge" title="Pinned">📌</span>}
          {file.is_favorite && <span className="pinned-badge" title="Favorite">⭐</span>}
        </div>
      </td>
      <td className="file-size">{file.file_type === 'directory' ? '-' : formatSize(file.size)}</td>
      <td className="file-date">{formatDate(file.created_at)}</td>
    </tr>
  );
}
