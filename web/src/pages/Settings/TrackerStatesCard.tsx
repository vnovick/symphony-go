import { useEffect, useRef, useState } from 'react';
import { SAVE_OK_BANNER_MS } from '../../utils/timings';
import { TagInput } from '../../components/symphony/TagInput';

interface TrackerStatesCardProps {
  initialActiveStates: string[];
  initialTerminalStates: string[];
  initialCompletionState: string;
  onSave: (
    activeStates: string[],
    terminalStates: string[],
    completionState: string,
  ) => Promise<boolean>;
}

export function TrackerStatesCard({
  initialActiveStates,
  initialTerminalStates,
  initialCompletionState,
  onSave,
}: TrackerStatesCardProps) {
  const [activeStates, setActiveStates] = useState<string[]>(initialActiveStates);
  const [terminalStates, setTerminalStates] = useState<string[]>(initialTerminalStates);
  const [completionState, setCompletionState] = useState<string>(initialCompletionState);
  const [statesSaving, setStatesSaving] = useState(false);
  const [statesSaveError, setStatesSaveError] = useState('');
  const [statesSaveOk, setStatesSaveOk] = useState(false);
  const saveOkTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (saveOkTimerRef.current !== null) clearTimeout(saveOkTimerRef.current);
    };
  }, []);

  const handleSaveStates = async () => {
    setStatesSaving(true);
    setStatesSaveError('');
    setStatesSaveOk(false);
    const ok = await onSave(activeStates, terminalStates, completionState);
    setStatesSaving(false);
    if (ok) {
      setStatesSaveOk(true);
      saveOkTimerRef.current = setTimeout(() => {
        setStatesSaveOk(false);
        saveOkTimerRef.current = null;
      }, SAVE_OK_BANNER_MS);
    } else {
      setStatesSaveError('Failed to save. Check the server logs.');
    }
  };

  return (
    <div
      className="overflow-hidden rounded-[var(--radius-md)]"
      style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)' }}
    >
      {/* Card header */}
      <div
        className="border-b px-5 py-4"
        style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)' }}
      >
        <h2 className="text-sm font-semibold" style={{ color: 'var(--text)' }}>
          Tracker States
        </h2>
        <p className="mt-0.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
          Configure which states the orchestrator picks up (Active), marks as done (Terminal), and
          transitions to on completion. Changes are written back to WORKFLOW.md.
        </p>
      </div>

      <div className="space-y-5 px-5 py-5">
        <div>
          <label
            className="mb-2 block text-xs font-medium tracking-wider uppercase"
            style={{ color: 'var(--muted)' }}
          >
            Active States
          </label>
          <TagInput
            chips={activeStates}
            onChange={setActiveStates}
            chipClassName="bg-[var(--accent-soft)] text-[var(--accent-strong)]"
            addButtonClassName="bg-[var(--accent-soft)] text-[var(--accent-strong)] hover:opacity-80"
          />
        </div>

        <div>
          <label
            className="mb-2 block text-xs font-medium tracking-wider uppercase"
            style={{ color: 'var(--muted)' }}
          >
            Terminal States
          </label>
          <TagInput
            chips={terminalStates}
            onChange={setTerminalStates}
            chipClassName="bg-[var(--bg-soft)] text-[var(--text-secondary)]"
            addButtonClassName="bg-[var(--bg-soft)] text-[var(--text-secondary)] hover:opacity-80"
          />
        </div>

        <div>
          <label
            className="mb-2 block text-xs font-medium tracking-wider uppercase"
            style={{ color: 'var(--muted)' }}
          >
            Completion State
          </label>
          <input
            type="text"
            value={completionState}
            onChange={(e) => { setCompletionState(e.target.value); }}
            placeholder="e.g. In Review (leave empty to skip)"
            className="w-64 rounded-[var(--radius-sm)] border px-3 py-2 text-[13px] focus:outline-none"
            style={{
              borderColor: 'var(--line)',
              background: 'var(--panel-strong)',
              color: 'var(--text)',
            }}
          />
          <p className="mt-1 text-xs" style={{ color: 'var(--muted)' }}>
            The state the agent moves an issue to when it finishes successfully. Has to be 1:1 with
            a tracker state.
          </p>
        </div>

        <div className="flex items-center gap-3 pt-1">
          <button
            onClick={handleSaveStates}
            disabled={statesSaving}
            className="rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
            style={{ background: 'var(--accent)' }}
          >
            {statesSaving ? 'Saving…' : 'Save Changes'}
          </button>
          {statesSaveOk && (
            <span className="text-sm" style={{ color: 'var(--success)' }}>
              Saved successfully.
            </span>
          )}
          {statesSaveError && (
            <span className="text-sm" style={{ color: 'var(--danger)' }}>
              {statesSaveError}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
