import { useToastStore } from '../../store/toastStore';

function toastStyle(variant: string | undefined) {
  if (variant === 'error') return { borderColor: 'var(--danger-soft)', background: 'var(--danger-soft)', color: 'var(--danger)' };
  if (variant === 'success') return { borderColor: 'var(--success-soft)', background: 'var(--success-soft)', color: 'var(--success)' };
  return { borderColor: 'var(--accent-soft)', background: 'var(--accent-soft)', color: 'var(--accent-strong)' };
}

/** Renders auto-dismissing toast notifications anchored to the bottom-right corner. */
export default function Toast() {
  const toasts = useToastStore((s) => s.toasts);
  const removeToast = useToastStore((s) => s.removeToast);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed right-4 bottom-4 z-50 flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          role="alert"
          className="flex min-w-[240px] max-w-sm items-start gap-3 rounded-[var(--radius-md)] border px-4 py-3 text-sm shadow-lg"
          style={toastStyle(t.variant)}
        >
          <span className="flex-1">{t.message}</span>
          <button
            onClick={() => { removeToast(t.id); }}
            aria-label="Dismiss notification"
            className="shrink-0 opacity-60 transition-opacity hover:opacity-100"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
