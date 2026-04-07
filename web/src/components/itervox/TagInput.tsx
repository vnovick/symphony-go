import { useState } from 'react';

interface TagInputProps {
  chips: string[];
  onChange: (chips: string[]) => void;
  /** Tailwind classes applied to each chip span. */
  chipClassName: string;
  /** Tailwind classes applied to the Add button. */
  addButtonClassName: string;
}

/**
 * Reusable tag-input: a chip list with an inline text input to add entries
 * and a remove button on each chip.
 */
export function TagInput({ chips, onChange, chipClassName, addButtonClassName }: TagInputProps) {
  const [inputValue, setInputValue] = useState('');

  const add = () => {
    const value = inputValue.trim();
    if (value && !chips.includes(value)) onChange([...chips, value]);
    setInputValue('');
  };

  const remove = (chip: string) => {
    onChange(chips.filter((c) => c !== chip));
  };

  return (
    <div className="flex flex-wrap gap-2">
      {chips.map((chip) => (
        <span
          key={chip}
          className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${chipClassName}`}
        >
          {chip}
          <button
            onClick={() => {
              remove(chip);
            }}
            className="ml-0.5 transition-opacity hover:opacity-60"
            title={`Remove ${chip}`}
          >
            ×
          </button>
        </span>
      ))}
      <span className="inline-flex items-center gap-1">
        <input
          type="text"
          value={inputValue}
          onChange={(e) => {
            setInputValue(e.target.value);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') add();
          }}
          placeholder="+ Add state"
          className="w-28 rounded border px-2 py-0.5 text-xs focus:ring-1 focus:outline-none"
          style={{ borderColor: 'var(--line)', background: 'var(--panel)', color: 'var(--text)' }}
        />
        {inputValue.trim() && (
          <button
            onClick={add}
            className={`rounded px-2 py-0.5 text-xs transition-colors ${addButtonClassName}`}
          >
            Add
          </button>
        )}
      </span>
    </div>
  );
}
