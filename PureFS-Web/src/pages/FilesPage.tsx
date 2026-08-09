import { useState, useEffect, useCallback, useRef } from 'react';
import { useUIStore } from '../store';
import { files } from '../api';
import type { FileItem, FileListResponse } from '../api';
import FileRow from '../components/FileRow';
import FileGrid from '../components/FileGrid';
import Breadcrumb from '../components/Breadcrumb';
import SelectionToolbar from '../components/SelectionToolbar';
import UploadOverlay from '../components/UploadOverlay';
import ContextMenu from '../components/ContextMenu';
import ShareDialog from '../components/ShareDialog';
import FilePreview from '../components/FilePreview';

interface BreadcrumbItem {
  id: number | null;
  name: string;
}

export default function FilesPage({ viewFilter }: { viewFilter?: string }) {
  const viewMode = useUIStore((s) => s.viewMode);
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);

  const [data, setData] = useState<FileListResponse>({ items: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [currentParentId, setCurrentParentId] = useState<number | null>(null);
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>(() => [
    { id: null, name: viewFilter === 'favorites' ? 'Favorites' : viewFilter === 'pinned' ? 'Pinned' : viewFilter === 'recent' ? 'Recent' : 'All Files' }
  ]);
  const [search, setSearch] = useState('');
  const [sortBy, setSortBy] = useState('created_at');
  const [sortOrder, setSortOrder] = useState('DESC');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [uploads, setUploads] = useState<{ name: string; progress: number; done: boolean }[]>([]);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; file: FileItem } | null>(null);
  const [shareFile, setShareFile] = useState<FileItem | null>(null);
  const [previewFile, setPreviewFile] = useState<FileItem | null>(null);
  const [renamingId, setRenamingId] = useState<number | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);
  const newFolderRef = useRef<HTMLInputElement>(null);

  // Reset navigation state when view filter changes (e.g. switching between /favorites and /pinned)
  useEffect(() => {
    const rootName = viewFilter === 'favorites' ? 'Favorites' : viewFilter === 'pinned' ? 'Pinned' : viewFilter === 'recent' ? 'Recent' : 'All Files';
    setBreadcrumbs([{ id: null, name: rootName }]);
    setCurrentParentId(null);
    setSelectedIds(new Set());
  }, [viewFilter]);

  const fetchFiles = useCallback(async () => {
    setLoading(true);
    try {
      const params: any = {
        parent_id: currentParentId ?? undefined,
        sort_by: sortBy,
        sort_order: sortOrder,
        search: search || undefined,
      };

      // Apply view filters
      if (viewFilter === 'favorites') params.is_favorite = true;
      if (viewFilter === 'pinned') params.is_pinned = true;
      if (viewFilter === 'recent') { params.sort_by = 'created_at'; params.sort_order = 'DESC'; }

      const result = await files.list(params);
      setData({ items: result.items, total: result.total });
    } catch {
      setData({ items: [], total: 0 });
    } finally {
      setLoading(false);
    }
  }, [currentParentId, sortBy, sortOrder, search, viewFilter]);

  useEffect(() => { fetchFiles(); }, [fetchFiles]);

  const handleNavigate = (file: FileItem) => {
    if (file.file_type !== 'directory') {
      setPreviewFile(file);
      setSelectedIds(new Set());
      return;
    }
    setCurrentParentId(file.id);
    setBreadcrumbs((prev) => [...prev, { id: file.id, name: file.name }]);
    setSelectedIds(new Set());
  };

  const handleBreadcrumb = (index: number) => {
    const target = breadcrumbs[index];
    setCurrentParentId(target.id);
    setBreadcrumbs(breadcrumbs.slice(0, index + 1));
    setSelectedIds(new Set());
  };

  const handleSort = (column: string) => {
    if (sortBy === column) {
      setSortOrder(sortOrder === 'ASC' ? 'DESC' : 'ASC');
    } else {
      setSortBy(column);
      setSortOrder('DESC');
    }
  };

  const handleSelect = (id: number, multi?: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(multi ? prev : []);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleDelete = async () => {
    for (const id of selectedIds) {
      try { await files.delete(id); } catch { /* silently skip failed deletes */ }
    }
    setSelectedIds(new Set());
    fetchFiles();
  };

  const handleRenameStart = (file: FileItem) => {
    setRenamingId(file.id);
    setRenameValue(file.name);
  };

  const handleRenameSubmit = async (id: number) => {
    const name = renameValue.trim();
    if (name) {
      try {
        await files.rename(id, name);
        fetchFiles();
      } catch { /* rename failed — keep old name */ }
    }
    setRenamingId(null);
  };

  const handleUploadFiles = async (fileList: FileList) => {
    const newUploads = [...uploads];
    for (let i = 0; i < fileList.length; i++) {
      const file = fileList[i];
      const item = { name: file.name, progress: 0, done: false };
      newUploads.push(item);
      setUploads([...newUploads]);
      try {
        await files.upload(file, currentParentId ?? undefined);
        item.progress = 100;
        item.done = true;
      } catch {
        item.done = true;
      }
      setUploads([...newUploads]);
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
    fetchFiles();
    // Clear uploads after 2 seconds
    setTimeout(() => setUploads([]), 2000);
  };

  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

  const handleCreateFolder = () => {
    setNewFolderName('');
    setShowNewFolder(true);
    setTimeout(() => newFolderRef.current?.focus(), 50);
  };

  const handleNewFolderSubmit = async () => {
    if (!newFolderName.trim()) return;
    try {
      await files.createDir({
        parent_id: currentParentId ?? undefined,
        name: newFolderName.trim(),
        file_type: 'directory',
      });
      setShowNewFolder(false);
      setNewFolderName('');
      fetchFiles();
    } catch {
      setShowNewFolder(false);
    }
  };

  const handleContextMenu = (e: React.MouseEvent, file: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, file });
    return false;
  };

  const handleRefresh = () => fetchFiles();

  const handleDownload = async (file: FileItem) => {
    if (file.file_type === 'directory') return;
    try {
      const blob = await files.downloadBlob(file.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = file.name;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch {
      /* download failed — silently ignore */
    }
  };

  const handleShare = (file: FileItem) => {
    setShareFile(file);
  };

  const handleFavorite = async (file: FileItem) => {
    try { await files.setFavorite(file.id, !file.is_favorite); }
    catch { return; }
    fetchFiles();
  };

  const handlePin = async (file: FileItem) => {
    try { await files.setPinned(file.id, !file.is_pinned); }
    catch { return; }
    fetchFiles();
  };

  return (
    <div className="main-content">
      {/* Header */}
      <div className="header">
        <div className="header-left">
          <button className="btn-icon" onClick={toggleSidebar} title="Toggle sidebar">☰</button>
          <button className="btn-icon" onClick={handleRefresh} title="Refresh">↻</button>
          <div className="search-box">
            <span style={{ color: 'var(--text-tertiary)' }}>🔍</span>
            <input placeholder="Search files..." value={search} onChange={(e) => setSearch(e.target.value)} />
          </div>
        </div>
        <div className="header-right">
          <button className="btn" onClick={handleCreateFolder}>+ New Folder</button>
          <button className="btn btn-primary" onClick={handleUploadClick}>+ Upload</button>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            style={{ display: 'none' }}
            onChange={(e) => e.target.files && handleUploadFiles(e.target.files)}
          />
          <div className="view-toggle">
            <button className={viewMode === 'list' ? 'active' : ''} onClick={() => useUIStore.getState().setViewMode('list')}>📋</button>
            <button className={viewMode === 'grid' ? 'active' : ''} onClick={() => useUIStore.getState().setViewMode('grid')}>▦</button>
            <button className={viewMode === 'timeline' ? 'active' : ''} onClick={() => useUIStore.getState().setViewMode('timeline')}>⏱</button>
          </div>
        </div>
      </div>

      {/* Breadcrumb */}
      <Breadcrumb items={breadcrumbs} onNavigate={handleBreadcrumb} />

      {/* Selection toolbar */}
      {selectedIds.size > 0 && (
        <SelectionToolbar
          count={selectedIds.size}
          onDelete={handleDelete}
          onClear={() => setSelectedIds(new Set())}
        />
      )}

      {/* File area */}
      <div className="file-area"
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => { e.preventDefault(); if (e.dataTransfer.files.length) handleUploadFiles(e.dataTransfer.files); }}
      >
        {showNewFolder && (
          <div style={{ padding: '8px 12px', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="file-icon" style={{ fontSize: 18 }}>📁</span>
            <input
              ref={newFolderRef}
              value={newFolderName}
              onChange={(e) => setNewFolderName(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleNewFolderSubmit(); if (e.key === 'Escape') setShowNewFolder(false); }}
              onBlur={handleNewFolderSubmit}
              placeholder="Folder name"
              style={{
                padding: '4px 8px', fontSize: 14, border: '1px solid var(--accent)',
                borderRadius: 'var(--radius-sm)', background: 'var(--bg-primary)',
                color: 'var(--text-primary)', outline: 'none', width: 200,
              }}
            />
          </div>
        )}
        {loading ? (
          <div className="empty-state">
            <div className="loading-spinner" style={{ width: 32, height: 32 }} />
            <p style={{ marginTop: 12 }}>Loading...</p>
          </div>
        ) : !data?.items || data.items.length === 0 ? (
          <div className="empty-state">
            <div className="icon">{viewFilter === 'favorites' ? '⭐' : viewFilter === 'pinned' ? '📌' : viewFilter === 'recent' ? '🕐' : '📁'}</div>
            <h3>
              {viewFilter === 'favorites' ? 'No favorites yet' :
               viewFilter === 'pinned' ? 'No pinned files yet' :
               viewFilter === 'recent' ? 'No recent files' :
               'No files yet'}
            </h3>
            <p>
              {viewFilter === 'favorites' ? 'Right-click a file and mark it as favorite.' :
               viewFilter === 'pinned' ? 'Right-click a file and pin it.' :
               viewFilter === 'recent' ? '' :
               'Drag and drop files here or use the upload button'}
            </p>
          </div>
        ) : viewMode === 'list' ? (
          <table className="file-table">
            <thead>
              <tr>
                <th style={{ width: 30 }}></th>
                <th onClick={() => handleSort('name')}>Name {sortBy === 'name' && (sortOrder === 'ASC' ? '↑' : '↓')}</th>
                <th style={{ width: 100 }} onClick={() => handleSort('size')}>Size</th>
                <th style={{ width: 180 }} onClick={() => handleSort('created_at')}>Date</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((file) => (
                <FileRow
                  key={file.id}
                  file={file}
                  selected={selectedIds.has(file.id)}
                  onSelect={(multi) => handleSelect(file.id, multi)}
                  onDoubleClick={() => handleNavigate(file)}
                  renaming={renamingId === file.id}
                  renameValue={renamingId === file.id ? renameValue : undefined}
                  onRenameChange={setRenameValue}
                  onRenameSubmit={() => handleRenameSubmit(file.id)}
                  onContextMenu={(e) => handleContextMenu(e, file)}
                />
              ))}
            </tbody>
          </table>
        ) : viewMode === 'grid' ? (
          <FileGrid
            files={data.items}
            onDoubleClick={handleNavigate}
            onContextMenu={handleContextMenu}
          />
        ) : (
          <TimelineView files={data.items} onDoubleClick={handleNavigate} onContextMenu={handleContextMenu} />
        )}
      </div>

      {/* Upload overlay */}
      {uploads.length > 0 && <UploadOverlay items={uploads} />}

      {/* Context menu */}
      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          file={contextMenu.file}
          onClose={() => setContextMenu(null)}
          onRename={() => { setRenamingId(contextMenu.file.id); setRenameValue(contextMenu.file.name); }}
          onDownload={() => { setContextMenu(null); handleDownload(contextMenu.file); }}
          onShare={() => { setContextMenu(null); handleShare(contextMenu.file); }}
          onFavorite={() => { setContextMenu(null); handleFavorite(contextMenu.file); }}
          onPin={() => { setContextMenu(null); handlePin(contextMenu.file); }}
          onDelete={() => { setContextMenu(null); setSelectedIds(new Set([contextMenu.file.id])); handleDelete().then(() => fetchFiles()); }}
          onRefresh={() => { setContextMenu(null); fetchFiles(); }}
        />
      )}

      {/* Share dialog */}
      {shareFile && (
        <ShareDialog
          file={shareFile}
          onClose={() => setShareFile(null)}
        />
      )}

      {/* File preview */}
      {previewFile && (
        <FilePreview
          file={previewFile}
          onClose={() => setPreviewFile(null)}
        />
      )}
    </div>
  );
}

function TimelineView({ files, onDoubleClick, onContextMenu }: {
  files: FileItem[];
  onDoubleClick: (f: FileItem) => void;
  onContextMenu: (e: React.MouseEvent, f: FileItem) => void;
}) {
  const groups = new Map<string, FileItem[]>();
  for (const f of files) {
    const date = f.created_at.split('T')[0];
    if (!groups.has(date)) groups.set(date, []);
    groups.get(date)!.push(f);
  }

  return (
    <div className="timeline-view">
      {Array.from(groups.entries()).map(([date, items]) => (
        <div key={date} className="timeline-group">
          <div className="timeline-date">{date}</div>
          {items.map((f) => (
            <div
              key={f.id}
              className="timeline-item"
              onDoubleClick={() => onDoubleClick(f)}
              onContextMenu={(e) => onContextMenu(e, f)}
            >
              <span className="file-icon">
                {f.file_type === 'directory' ? '📁' : f.mime_type?.startsWith('image/') ? '🖼️' : '📄'}
              </span>
              <span className="file-name">{f.name}</span>
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
