import { memo } from 'react';

export interface FilterPill {
  id: string;
  label: string;
  /** State names this pill matches. Empty = match all. */
  states: string[];
}

interface FilterPillsProps {
  pills: FilterPill[];
  activeId: string;
  onChange: (id: string) => void;
}

/**
 * Linear-style horizontal filter pills.
 * "All Issues" matches everything; other pills match specific state groups.
 */
export const FilterPills = memo(function FilterPills({
  pills,
  activeId,
  onChange,
}: FilterPillsProps) {
  return (
    <div className="flex items-center gap-1.5 overflow-x-auto">
      {pills.map((pill) => (
        <button
          key={pill.id}
          onClick={() => { onChange(pill.id); }}
          className={`flex-shrink-0 rounded-full px-3 py-1 text-xs font-medium transition-colors ${
            activeId === pill.id
              ? 'bg-theme-accent text-white'
              : 'bg-theme-bg-soft text-theme-text-secondary hover:text-theme-text'
          }`}
        >
          {pill.label}
        </button>
      ))}
    </div>
  );
});
