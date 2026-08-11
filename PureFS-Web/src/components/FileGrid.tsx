import { files } from '../api';
import type { FileItem } from '../api';

interface FileGridProps {
  files: FileItem[];
  onDoubleClick: (f: FileItem) => void;
  onContextMenu: (e: React.MouseEvent, f: FileItem) => void;
}

export default function FileGrid({ files, onDoubleClick, onContextMenu }: FileGridProps) {
  return (
    <div className="grid-view">
      {files.map((f) => (
        <div
          key={f.id}
          className="grid-item"
          onDoubleClick={() => onDoubleClick(f)}
          onContextMenu={(e) => onContextMenu(e, f)}
        >
          <div className="file-icon">
            {f.file_type === 'directory' ? (
              <span style={{ fontSize: 48 }}>📁</span>
            ) : f.is_e2ee ? (
              <span style={{ fontSize: 36 }} title="End-to-end encrypted">🔒</span>
            ) : f.mime_type?.startsWith('image/') ? (
              <img
                src={`${files.downloadBlobUrl(f.id)}`}
                alt={f.name}
                loading="lazy"
                style={{ width: 80, height: 80, objectFit: 'cover', borderRadius: 6 }}
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
              />
            ) : (
              <span style={{ fontSize: 36 }}>
                {f.mime_type?.startsWith('video/') ? '🎬' :
                 f.mime_type?.startsWith('audio/') ? '🎵' :
                 f.mime_type?.includes('pdf') ? '📕' :
                 f.mime_type?.includes('zip') || f.mime_type?.includes('rar') || f.mime_type?.includes('tar') ? '📦' : '📄'}
              </span>
            )}
          </div>
          <div className="file-name" title={f.name}>{f.name}</div>
        </div>
      ))}
    </div>
  );
}
