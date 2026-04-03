interface BackendSelectorProps {
  value: string;
  onChange: (backend: string) => void;
  /** When true, renders a read-only badge instead of a dropdown. */
  readOnly?: boolean;
  /** Show "Backend:" label. Default true. */
  showLabel?: boolean;
  size?: 'sm' | 'md';
}

const SIZE_CLS = {
  sm: 'px-1.5 py-0.5 text-[10px]',
  md: 'px-2.5 py-1.5 text-xs',
} as const;

export function BackendSelector({
  value,
  onChange,
  readOnly = false,
  showLabel = true,
  size = 'md',
}: BackendSelectorProps) {
  if (readOnly) {
    return (
      <span className={`rounded-full font-medium bg-theme-bg-soft text-theme-text-secondary ${SIZE_CLS[size]}`}>
        {value}
      </span>
    );
  }

  const select = (
    <select
      value={value}
      onChange={(e) => { onChange(e.target.value); }}
      onClick={(e) => { e.stopPropagation(); }}
      className={`rounded-[var(--radius-sm)] border border-theme-line bg-theme-panel-strong text-theme-text font-medium focus:outline-none cursor-pointer ${SIZE_CLS[size]}`}
    >
      <option value="claude">Claude</option>
      <option value="codex">Codex</option>
    </select>
  );

  if (!showLabel) return select;

  return (
    <label className="flex items-center gap-1 flex-shrink-0 text-[10px] text-theme-muted" onClick={(e) => { e.stopPropagation(); }}>
      Backend:
      {select}
    </label>
  );
}
