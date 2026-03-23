import { useState } from 'react';

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
  const [statesAddActive, setStatesAddActive] = useState('');
  const [statesAddTerminal, setStatesAddTerminal] = useState('');
  const [statesSaving, setStatesSaving] = useState(false);
  const [statesSaveError, setStatesSaveError] = useState('');
  const [statesSaveOk, setStatesSaveOk] = useState(false);

  const handleSaveStates = async () => {
    setStatesSaving(true);
    setStatesSaveError('');
    setStatesSaveOk(false);
    const ok = await onSave(activeStates, terminalStates, completionState);
    setStatesSaving(false);
    if (ok) {
      setStatesSaveOk(true);
      setTimeout(() => {
        setStatesSaveOk(false);
      }, 3000);
    } else {
      setStatesSaveError('Failed to save. Check the server logs.');
    }
  };

  return (
    <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
      <div className="border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Tracker States</h2>
        <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          Configure which states the orchestrator picks up (Active), marks as done (Terminal), and
          transitions to on completion. Changes are written back to WORKFLOW.md.
        </p>
      </div>
      <div className="space-y-5 px-6 py-5">
        <div>
          <label className="mb-2 block text-xs font-medium tracking-wider text-gray-600 uppercase dark:text-gray-400">
            Active States
          </label>
          <div className="mb-2 flex flex-wrap gap-2">
            {activeStates.map((s) => (
              <span
                key={s}
                className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900/30 dark:text-blue-300"
              >
                {s}
                <button
                  onClick={() => {
                    setActiveStates(activeStates.filter((x) => x !== s));
                  }}
                  className="ml-0.5 transition-colors hover:text-red-500 dark:hover:text-red-400"
                  title={`Remove ${s}`}
                >
                  ×
                </button>
              </span>
            ))}
            <span className="inline-flex items-center gap-1">
              <input
                type="text"
                value={statesAddActive}
                onChange={(e) => {
                  setStatesAddActive(e.target.value);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && statesAddActive.trim()) {
                    const value = statesAddActive.trim();
                    if (!activeStates.includes(value)) setActiveStates([...activeStates, value]);
                    setStatesAddActive('');
                  }
                }}
                placeholder="+ Add state"
                className="focus:ring-brand-500 w-28 rounded border border-gray-300 bg-white px-2 py-0.5 text-xs text-gray-800 focus:ring-1 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              />
              {statesAddActive.trim() && (
                <button
                  onClick={() => {
                    const value = statesAddActive.trim();
                    if (!activeStates.includes(value)) setActiveStates([...activeStates, value]);
                    setStatesAddActive('');
                  }}
                  className="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-800 transition-colors hover:bg-blue-200 dark:bg-blue-900/30 dark:text-blue-300 dark:hover:bg-blue-900/50"
                >
                  Add
                </button>
              )}
            </span>
          </div>
        </div>

        <div>
          <label className="mb-2 block text-xs font-medium tracking-wider text-gray-600 uppercase dark:text-gray-400">
            Terminal States
          </label>
          <div className="mb-2 flex flex-wrap gap-2">
            {terminalStates.map((s) => (
              <span
                key={s}
                className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-gray-800 dark:text-gray-300"
              >
                {s}
                <button
                  onClick={() => {
                    setTerminalStates(terminalStates.filter((x) => x !== s));
                  }}
                  className="ml-0.5 transition-colors hover:text-red-500 dark:hover:text-red-400"
                  title={`Remove ${s}`}
                >
                  ×
                </button>
              </span>
            ))}
            <span className="inline-flex items-center gap-1">
              <input
                type="text"
                value={statesAddTerminal}
                onChange={(e) => {
                  setStatesAddTerminal(e.target.value);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && statesAddTerminal.trim()) {
                    const value = statesAddTerminal.trim();
                    if (!terminalStates.includes(value))
                      setTerminalStates([...terminalStates, value]);
                    setStatesAddTerminal('');
                  }
                }}
                placeholder="+ Add state"
                className="focus:ring-brand-500 w-28 rounded border border-gray-300 bg-white px-2 py-0.5 text-xs text-gray-800 focus:ring-1 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              />
              {statesAddTerminal.trim() && (
                <button
                  onClick={() => {
                    const value = statesAddTerminal.trim();
                    if (!terminalStates.includes(value))
                      setTerminalStates([...terminalStates, value]);
                    setStatesAddTerminal('');
                  }}
                  className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-700 transition-colors hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
                >
                  Add
                </button>
              )}
            </span>
          </div>
        </div>

        <div>
          <label className="mb-2 block text-xs font-medium tracking-wider text-gray-600 uppercase dark:text-gray-400">
            Completion State
          </label>
          <input
            type="text"
            value={completionState}
            onChange={(e) => {
              setCompletionState(e.target.value);
            }}
            placeholder="e.g. In Review (leave empty to skip)"
            className="focus:ring-brand-500 w-64 rounded border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-800 focus:ring-2 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">
            The state the agent moves an issue to when it finishes successfully. Has to be 1:1 with
            a tracker state.
          </p>
        </div>

        <div className="flex items-center gap-3 pt-1">
          <button
            onClick={handleSaveStates}
            disabled={statesSaving}
            className="bg-brand-500 hover:bg-brand-600 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
          >
            {statesSaving ? 'Saving…' : 'Save Changes'}
          </button>
          {statesSaveOk && (
            <span className="text-sm text-green-600 dark:text-green-400">Saved successfully.</span>
          )}
          {statesSaveError && (
            <span className="text-sm text-red-600 dark:text-red-400">{statesSaveError}</span>
          )}
        </div>
      </div>
    </div>
  );
}
