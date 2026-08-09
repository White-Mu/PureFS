import { createPortal } from 'react-dom';
import type { FileItem } from '../api';

interface ContextMenuProps {
  x: number;
  y: number;
  file: FileItem;
  onClose: () => void;
  onDownload: () => void;
  onRename: () => void;
  onShare: () => void;
  onDelete: () => void;
  onRefresh: () => void;
  onFavorite: () => void;
  onPin: () => void;
}

export default function ContextMenu(props: ContextMenuProps) {
  const { x, y, file, onClose, onDownload, onRename, onShare, onDelete, onRefresh, onFavorite, onPin } = props;

  const act = (fn: () => void) => (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    fn();
  };

  return createPortal(
    <div className="ctx-backdrop" onClick={onClose} onContextMenu={(e) => { e.preventDefault(); onClose(); }}>
      <div className="context-menu" style={{ position: 'fixed', left: x, top: y }}>
        <div className="context-menu-item" onClick={act(onRefresh)}>↻ Refresh</div>
        <div className="context-menu-divider" />
        <div className="context-menu-item" onClick={act(onFavorite)}>{file.is_favorite ? '⭐ Unfavorite' : '⭐ Favorite'}</div>
        <div className="context-menu-item" onClick={act(onPin)}>{file.is_pinned ? '📌 Unpin' : '📌 Pin'}</div>
        <div className="context-menu-divider" />
        <div className="context-menu-item" onClick={act(onRename)}>✏ Rename</div>
        <div className="context-menu-item" onClick={act(onDownload)}>⬇ Download</div>
        <div className="context-menu-item" onClick={act(onShare)}>🔗 Share</div>
        <div className="context-menu-divider" />
        <div className="context-menu-item danger" onClick={act(onDelete)}>🗑 Delete</div>
      </div>
    </div>,
    document.body,
  );
}
