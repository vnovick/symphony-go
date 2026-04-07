import { useState } from 'react';

interface ConfirmButtonProps {
  label: string;
  confirmLabel: string;
  pendingLabel: string;
  isPending: boolean;
  onConfirm: () => void;
}

/**
 * Two-step danger button: first click shows "Are you sure?" with Yes/Cancel,
 * second click triggers the action.
 */
export function ConfirmButton({
  label,
  confirmLabel,
  pendingLabel,
  isPending,
  onConfirm,
}: ConfirmButtonProps) {
  const [confirming, setConfirming] = useState(false);

  if (confirming) {
    return (
      <div className="flex items-center gap-2">
        <span className="text-theme-muted text-xs">Are you sure?</span>
        <button
          onClick={() => {
            onConfirm();
            setConfirming(false);
          }}
          disabled={isPending}
          style={{
            padding: '4px 10px',
            borderRadius: 4,
            fontSize: 12,
            fontWeight: 600,
            cursor: isPending ? 'wait' : 'pointer',
            background: 'var(--danger)',
            color: '#fff',
            border: 'none',
          }}
        >
          {isPending ? pendingLabel : confirmLabel}
        </button>
        <button
          onClick={() => {
            setConfirming(false);
          }}
          style={{
            padding: '4px 10px',
            borderRadius: 4,
            fontSize: 12,
            cursor: 'pointer',
            background: 'transparent',
            color: 'var(--text-secondary)',
            border: '1px solid var(--line)',
          }}
        >
          Cancel
        </button>
      </div>
    );
  }

  return (
    <button
      onClick={() => {
        setConfirming(true);
      }}
      style={{
        padding: '6px 14px',
        borderRadius: 4,
        fontSize: 12,
        fontWeight: 500,
        cursor: 'pointer',
        background: 'transparent',
        color: 'var(--danger)',
        border: '1px solid var(--danger)',
      }}
    >
      {label}
    </button>
  );
}
