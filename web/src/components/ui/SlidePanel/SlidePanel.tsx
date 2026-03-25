import { useEffect, useId, type ReactNode } from 'react';

type Direction = 'left' | 'right' | 'bottom';

interface SlidePanelProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  direction?: Direction;
}

const PANEL_CLASS: Record<Direction, string> = {
  right: 'inset-y-0 right-0 w-[75vw] slide-panel-right',
  left: 'inset-y-0 left-0 w-[75vw]',
  bottom: 'inset-x-0 bottom-0 h-auto max-h-[90vh]',
};

export function SlidePanel({ open, onClose, title, children, direction = 'right' }: SlidePanelProps) {
  const titleId = useId();

  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, onClose]);

  // Lock body scroll while open
  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex">
      {/* Overlay */}
      <div
        data-testid="slide-panel-overlay"
        className="absolute inset-0"
        style={{ background: 'rgba(0,0,0,0.5)' }}
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Panel */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={`absolute flex flex-col overflow-hidden ${PANEL_CLASS[direction]}`}
        style={{ background: 'var(--panel)', borderLeft: '1px solid var(--line)' }}
      >
        <div
          className="flex items-center justify-between px-4 py-3 border-b"
          style={{ borderColor: 'var(--line)' }}
        >
          <h2 id={titleId} className="font-semibold" style={{ color: 'var(--text)' }}>
            {title}
          </h2>
          <button
            onClick={onClose}
            aria-label="Close panel"
            className="flex items-center justify-center w-8 h-8 rounded-lg transition-colors"
            style={{ color: 'var(--text-secondary)' }}
          >
            ✕
          </button>
        </div>
        <div className="flex-1 min-h-0 flex flex-col">{children}</div>
      </div>
    </div>
  );
}
