interface UploadOverlayProps {
  items: { name: string; progress: number; done: boolean }[];
}

export default function UploadOverlay({ items }: UploadOverlayProps) {
  const pending = items.filter((i) => !i.done);
  if (pending.length === 0 && items.length > 0) return null;

  return (
    <div className="upload-overlay">
      <div style={{ padding: '8px 12px', fontWeight: 600, fontSize: 14 }}>
        {pending.length > 0 ? 'Uploading...' : 'Complete'}
      </div>
      {items.map((item, idx) => (
        <div key={idx} className="upload-progress-item">
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <span>{item.name}</span>
            <span>{item.done ? '✓' : `${item.progress}%`}</span>
          </div>
          <div className="upload-progress-bar" style={{ width: `${item.progress}%` }} />
        </div>
      ))}
    </div>
  );
}
