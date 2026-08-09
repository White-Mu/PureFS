import type { FileItem } from '../api';

interface FileGridProps {
  files: FileItem[];
  onDoubleClick: (f: FileItem) => void;
  onContextMenu: (e: React.MouseEvent, f: FileItem) => void;
}

export default function FileGrid({ files, onDoubleClick, onContextMenu }: FileGridProps) {
  const icon = (f: FileItem) => f.file_type === 'directory' ? '📁' :
    f.mime_type?.startsWith('image/') ? '🖼️' :
    f.mime_type?.startsWith('video/') ? '🎬' :
    f.mime_type?.startsWith('audio/') ? '🎵' :
    f.mime_type?.includes('pdf') ? '📕' :
    f.mime_type?.includes('zip') || f.mime_type?.includes('rar') || f.mime_type?.includes('tar') ? '📦' : '📄';

  return (
    <div className="grid-view">
      {files.map((f) => (
        <div
          key={f.id}
          className="grid-item"
          onDoubleClick={() => onDoubleClick(f)}
          onContextMenu={(e) => onContextMenu(e, f)}
        >
          <div className="file-icon">{icon(f)}</div>
          <div className="file-name">{f.name}</div>
        </div>
      ))}
    </div>
  );
}
