interface SelectionToolbarProps {
  count: number;
  onDelete: () => void;
  onClear: () => void;
}

export default function SelectionToolbar({ count, onDelete, onClear }: SelectionToolbarProps) {
  return (
    <div className="selection-toolbar">
      <span className="count">{count} selected</span>
      <button className="btn btn-danger" onClick={onDelete}>Delete</button>
      <button className="btn btn-ghost" onClick={onClear}>Clear selection</button>
    </div>
  );
}
