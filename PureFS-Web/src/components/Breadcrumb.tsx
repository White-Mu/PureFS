interface BreadcrumbItem {
  id: number | null;
  name: string;
}

interface BreadcrumbProps {
  items: BreadcrumbItem[];
  onNavigate: (index: number) => void;
}

export default function Breadcrumb({ items, onNavigate }: BreadcrumbProps) {
  return (
    <div className="breadcrumb">
      {items.map((item, i) => (
        <span key={i} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          {i > 0 && <span className="breadcrumb-sep">/</span>}
          <span
            className={i === items.length - 1 ? 'breadcrumb-current' : 'breadcrumb-item'}
            onClick={() => onNavigate(i)}
          >
            {item.name}
          </span>
        </span>
      ))}
    </div>
  );
}
