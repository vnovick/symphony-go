import { useState, useEffect } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';
import PageMeta from '../../components/common/PageMeta';
import type { ProfileDef } from '../../types/symphony';

const EMPTY_PROFILE_DEFS: Record<string, ProfileDef> = {};

const CLAUDE_MODELS = [
  { id: 'claude-haiku-4-5-20251001', label: 'Haiku 4.5  — Fast' },
  { id: 'claude-sonnet-4-6', label: 'Sonnet 4.6 — Balanced' },
  { id: 'claude-opus-4-6', label: 'Opus 4.6  — Powerful' },
];

const MODELS_DATALIST_ID = 'claude-models-datalist';

function commandToModel(cmd: string | undefined | null): string {
  if (!cmd) return '';
  const m = cmd.match(/--model\s+(\S+)/);
  return m ? m[1] : '';
}

function modelToCommand(model: string): string {
  const trimmed = model.trim();
  return trimmed ? `claude --model ${trimmed}` : 'claude';
}

function modelLabel(modelId: string): string {
  return CLAUDE_MODELS.find((m) => m.id === modelId)?.label ?? modelId;
}

function ModelInput({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <>
      <datalist id={MODELS_DATALIST_ID}>
        {CLAUDE_MODELS.map((m) => (
          <option key={m.id} value={m.id}>
            {m.label}
          </option>
        ))}
      </datalist>
      <input
        list={MODELS_DATALIST_ID}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        placeholder="Model ID (e.g. claude-sonnet-4-6) or leave blank for default"
        className={selectCls}
      />
    </>
  );
}

const selectCls =
  'w-full bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded px-3 py-1.5 text-sm text-gray-800 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-brand-500';
const textareaCls =
  'w-full bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded px-3 py-1.5 text-xs text-gray-800 dark:text-gray-100 font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 resize-y min-h-[56px]';

interface ProfileRowProps {
  name: string;
  def: ProfileDef;
  onEdit: (name: string, def: ProfileDef) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
}

function ProfileRow({ name, def, onEdit, onDelete }: ProfileRowProps) {
  const [editing, setEditing] = useState(false);
  const [editModel, setEditModel] = useState(() => commandToModel(def.command));
  const [editPrompt, setEditPrompt] = useState(def.prompt ?? '');
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    await onEdit(name, {
      command: modelToCommand(editModel),
      prompt: editPrompt.trim(),
    });
    setSaving(false);
    setEditing(false);
  };

  const handleCancel = () => {
    setEditModel(commandToModel(def.command));
    setEditPrompt(def.prompt ?? '');
    setEditing(false);
  };

  if (editing) {
    return (
      <tr className="border-b border-gray-200 bg-gray-50/50 dark:border-gray-700 dark:bg-gray-800/20">
        <td className="px-4 py-3 align-top font-mono text-sm text-gray-700 dark:text-gray-200">
          {name}
        </td>
        <td className="space-y-2 px-4 py-3">
          <ModelInput value={editModel} onChange={setEditModel} />
          <textarea
            className={textareaCls}
            placeholder="Agent role / description (optional, shown to orchestrating agent when teams are enabled)"
            value={editPrompt}
            onChange={(e) => {
              setEditPrompt(e.target.value);
            }}
          />
        </td>
        <td className="px-4 py-3 text-right align-top whitespace-nowrap">
          <button
            onClick={handleSave}
            disabled={saving}
            className="bg-brand-500 hover:bg-brand-600 mr-2 rounded-lg px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save'}
          </button>
          <button
            onClick={handleCancel}
            className="rounded-lg border border-gray-300 px-3 py-1 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            Cancel
          </button>
        </td>
      </tr>
    );
  }

  const modelId = commandToModel(def.command);
  const displayModel = modelId ? modelLabel(modelId) : 'Default';
  return (
    <tr className="border-b border-gray-200 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800/50">
      <td className="px-4 py-3 align-top font-mono text-sm text-gray-700 dark:text-gray-200">
        {name}
      </td>
      <td className="space-y-1 px-4 py-3 align-top">
        <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-gray-800 dark:text-gray-300">
          {displayModel}
        </span>
        {def.prompt && (
          <p
            className="max-w-xs truncate text-xs text-gray-400 italic dark:text-gray-500"
            title={def.prompt}
          >
            {def.prompt.slice(0, 80)}
            {def.prompt.length > 80 ? '…' : ''}
          </p>
        )}
      </td>
      <td className="px-4 py-3 text-right align-top whitespace-nowrap">
        {confirmDelete ? (
          <span className="inline-flex items-center gap-2">
            <span className="text-xs text-gray-500 dark:text-gray-400">Delete "{name}"?</span>
            <button
              onClick={async () => {
                setDeleting(true);
                await onDelete(name);
                setDeleting(false);
                setConfirmDelete(false);
              }}
              disabled={deleting}
              className="rounded-lg bg-red-600 px-3 py-1 text-sm text-white transition-colors hover:bg-red-700 disabled:opacity-50"
            >
              {deleting ? 'Deleting…' : 'Confirm'}
            </button>
            <button
              onClick={() => {
                setConfirmDelete(false);
              }}
              disabled={deleting}
              className="rounded-lg border border-gray-300 px-3 py-1 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
          </span>
        ) : (
          <>
            <button
              onClick={() => {
                setEditing(true);
              }}
              className="mr-2 rounded-lg border border-gray-300 px-3 py-1 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Edit
            </button>
            <button
              onClick={() => {
                setConfirmDelete(true);
              }}
              className="rounded-lg border border-red-200 px-3 py-1 text-sm text-red-600 transition-colors hover:bg-red-50 dark:border-red-800/60 dark:text-red-400 dark:hover:bg-red-900/20"
            >
              Delete
            </button>
          </>
        )}
      </td>
    </tr>
  );
}

export default function Settings() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const profileDefs = useSymphonyStore((s) => s.snapshot?.profileDefs ?? EMPTY_PROFILE_DEFS);
  const agentMode = useSymphonyStore((s) => s.snapshot?.agentMode ?? '');
  const { upsertProfile, deleteProfile, setAgentMode, updateTrackerStates } = useSettingsActions();

  // Tracker states editing
  const [activeStates, setActiveStates] = useState<string[]>([]);
  const [terminalStates, setTerminalStates] = useState<string[]>([]);
  const [completionState, setCompletionState] = useState<string>('');
  const [statesAddActive, setStatesAddActive] = useState('');
  const [statesAddTerminal, setStatesAddTerminal] = useState('');
  const [statesSaving, setStatesSaving] = useState(false);
  const [statesSaveError, setStatesSaveError] = useState('');
  const [statesSaveOk, setStatesSaveOk] = useState(false);

  // Sync local state from snapshot when it first loads or changes
  useEffect(() => {
    if (!snapshot) return;
    setActiveStates(snapshot.activeStates ?? []);
    setTerminalStates(snapshot.terminalStates ?? []);
    setCompletionState(snapshot.completionState ?? '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    snapshot?.activeStates?.join(','),
    snapshot?.terminalStates?.join(','),
    snapshot?.completionState,
  ]);

  const handleSaveStates = async () => {
    setStatesSaving(true);
    setStatesSaveError('');
    setStatesSaveOk(false);
    const ok = await updateTrackerStates(activeStates, terminalStates, completionState);
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

  const [agentModeToggling, setAgentModeToggling] = useState(false);

  const handleAgentModeChange = async (mode: string) => {
    setAgentModeToggling(true);
    await setAgentMode(mode);
    setAgentModeToggling(false);
  };

  const [adding, setAdding] = useState(false);
  const [newName, setNewName] = useState('');
  const [newModel, setNewModel] = useState('');
  const [newPrompt, setNewPrompt] = useState('');
  const [addSaving, setAddSaving] = useState(false);
  const [addError, setAddError] = useState('');

  const handleEdit = async (name: string, def: ProfileDef) => {
    await upsertProfile(name, def.command, def.prompt);
  };

  const [deleteError, setDeleteError] = useState('');

  const handleDelete = async (name: string) => {
    setDeleteError('');
    const ok = await deleteProfile(name);
    if (!ok) setDeleteError(`Failed to delete profile "${name}". Check the server logs.`);
  };

  const handleAdd = async () => {
    const trimName = newName.trim();
    if (!trimName) {
      setAddError('Profile name is required.');
      return;
    }
    if (/\s/.test(trimName)) {
      setAddError('Profile name must not contain spaces.');
      return;
    }
    setAddSaving(true);
    setAddError('');
    const ok = await upsertProfile(
      trimName,
      modelToCommand(newModel),
      newPrompt.trim() || undefined,
    );
    setAddSaving(false);
    if (ok) {
      setNewName('');
      setNewModel('');
      setNewPrompt('');
      setAdding(false);
    } else {
      setAddError('Failed to save profile. Check the server logs.');
    }
  };

  const handleAddCancel = () => {
    setNewName('');
    setNewModel('');
    setNewPrompt('');
    setAddError('');
    setAdding(false);
  };

  const profileEntries = Object.entries(profileDefs).sort(([a], [b]) => a.localeCompare(b));

  return (
    <>
      <PageMeta title="Simphony | Settings" description="Agent profile management for Simphony" />
      <div className="max-w-3xl space-y-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">
            Settings
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage agent profiles. Profiles are also hot-reloaded from{' '}
            <code className="text-brand-600 dark:text-brand-400 rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs dark:bg-gray-800">
              WORKFLOW.md
            </code>
            .
          </p>
        </div>

        <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
          <div className="flex items-center justify-between border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
            <div>
              <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                Agent Profiles
              </h2>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                Select per-issue from the issue detail modal. Each profile can override model,
                prompt, and sub-agent settings.
              </p>
            </div>
            {!adding && (
              <button
                onClick={() => {
                  setAdding(true);
                }}
                className="bg-brand-500 hover:bg-brand-600 flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors"
              >
                <svg
                  className="h-3.5 w-3.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
                </svg>
                Add Profile
              </button>
            )}
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 dark:bg-gray-900/50">
                <tr>
                  <th className="w-40 px-4 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase dark:text-gray-400">
                    Name
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium tracking-wider text-gray-500 uppercase dark:text-gray-400">
                    Model
                  </th>
                  <th className="w-40 px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                {profileEntries.map(([name, def]) => (
                  <ProfileRow
                    key={name}
                    name={name}
                    def={def}
                    onEdit={handleEdit}
                    onDelete={handleDelete}
                  />
                ))}

                {adding && (
                  <tr className="border-b border-gray-200 bg-gray-50/50 dark:border-gray-700 dark:bg-gray-800/30">
                    <td className="px-4 py-3 align-top">
                      <input
                        className="focus:ring-brand-500 w-full rounded border border-gray-300 bg-white px-3 py-1.5 font-mono text-sm text-gray-800 focus:ring-2 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                        placeholder="profile-name"
                        value={newName}
                        onChange={(e) => {
                          setNewName(e.target.value);
                        }}
                        onKeyDown={(e) => {
                          if (e.key === 'Escape') handleAddCancel();
                        }}
                        autoFocus
                      />
                    </td>
                    <td className="space-y-2 px-4 py-3">
                      <ModelInput value={newModel} onChange={setNewModel} />
                      <textarea
                        className={textareaCls}
                        placeholder="Agent role / description (shown to sub-agents). Leave empty if not using sub-agents."
                        value={newPrompt}
                        onChange={(e) => {
                          setNewPrompt(e.target.value);
                        }}
                      />
                    </td>
                    <td className="px-4 py-3 text-right align-top whitespace-nowrap">
                      <button
                        onClick={handleAdd}
                        disabled={addSaving}
                        className="bg-brand-500 hover:bg-brand-600 mr-2 rounded-lg px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
                      >
                        {addSaving ? 'Saving…' : 'Save'}
                      </button>
                      <button
                        onClick={handleAddCancel}
                        className="rounded-lg border border-gray-300 px-3 py-1 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                      >
                        Cancel
                      </button>
                    </td>
                  </tr>
                )}

                {profileEntries.length === 0 && !adding && (
                  <tr>
                    <td
                      colSpan={3}
                      className="px-4 py-10 text-center text-sm text-gray-400 dark:text-gray-500"
                    >
                      No profiles configured yet.{' '}
                      <button
                        onClick={() => {
                          setAdding(true);
                        }}
                        className="text-brand-500 hover:underline"
                      >
                        Add one
                      </button>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        {addError && <p className="text-sm text-red-600 dark:text-red-400">{addError}</p>}
        {deleteError && <p className="text-sm text-red-600 dark:text-red-400">{deleteError}</p>}

        {/* Agent Mode selector (Claude Code philosophy: Solo / Sub-agents / Agent Teams) */}
        <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
          <div className="border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Agent Mode</h2>
            <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              Choose how Claude collaborates. Solo: no profile context injected. Agent Teams:
              profile roles injected into the prompt so Claude knows which specialised agents to
              call.
            </p>
          </div>
          <div className="flex items-start justify-between gap-4 px-6 py-4">
            <div className="flex-1 space-y-1.5">
              {agentMode === '' && (
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  <span className="font-medium text-gray-700 dark:text-gray-200">Solo</span> —
                  Claude works alone with no sub-agent access.
                </p>
              )}
              {agentMode === 'teams' && (
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  <span className="font-medium text-gray-700 dark:text-gray-200">Agent Teams</span>{' '}
                  — profile roles injected into the prompt so Claude knows which specialised agents
                  it can call.{' '}
                  {profileEntries.length === 0 && (
                    <span className="text-amber-600 dark:text-amber-400">
                      Add profiles above to use this mode.
                    </span>
                  )}
                </p>
              )}
            </div>
            <select
              value={agentMode}
              disabled={agentModeToggling}
              onChange={(e) => handleAgentModeChange(e.target.value)}
              className="focus:ring-brand-500 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-800 focus:ring-2 focus:outline-none disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            >
              <option value="">Solo (default)</option>
              <option value="teams" disabled={profileEntries.length === 0}>
                Agent Teams
              </option>
            </select>
          </div>
        </div>

        {/* Tracker States */}
        <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
          <div className="border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
              Tracker States
            </h2>
            <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              Configure which states the orchestrator picks up (Active), marks as done (Terminal),
              and transitions to on completion. Changes are written back to WORKFLOW.md.
            </p>
          </div>
          <div className="space-y-5 px-6 py-5">
            {/* Active States */}
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
                        const val = statesAddActive.trim();
                        if (!activeStates.includes(val)) setActiveStates([...activeStates, val]);
                        setStatesAddActive('');
                      }
                    }}
                    placeholder="+ Add state"
                    className="focus:ring-brand-500 w-28 rounded border border-gray-300 bg-white px-2 py-0.5 text-xs text-gray-800 focus:ring-1 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  />
                  {statesAddActive.trim() && (
                    <button
                      onClick={() => {
                        const val = statesAddActive.trim();
                        if (!activeStates.includes(val)) setActiveStates([...activeStates, val]);
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

            {/* Terminal States */}
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
                        const val = statesAddTerminal.trim();
                        if (!terminalStates.includes(val))
                          setTerminalStates([...terminalStates, val]);
                        setStatesAddTerminal('');
                      }
                    }}
                    placeholder="+ Add state"
                    className="focus:ring-brand-500 w-28 rounded border border-gray-300 bg-white px-2 py-0.5 text-xs text-gray-800 focus:ring-1 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  />
                  {statesAddTerminal.trim() && (
                    <button
                      onClick={() => {
                        const val = statesAddTerminal.trim();
                        if (!terminalStates.includes(val))
                          setTerminalStates([...terminalStates, val]);
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

            {/* Completion State */}
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
                The state the agent moves an issue to when it finishes successfully. Has to be 1:1
                with a tracker state.
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
                <span className="text-sm text-green-600 dark:text-green-400">
                  Saved successfully.
                </span>
              )}
              {statesSaveError && (
                <span className="text-sm text-red-600 dark:text-red-400">{statesSaveError}</span>
              )}
            </div>
          </div>
        </div>

        {/* WORKFLOW.md reference */}
        <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
          <div className="border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
              WORKFLOW.md profile syntax
            </h2>
          </div>
          <div className="px-6 py-5">
            <pre className="overflow-x-auto rounded-xl bg-gray-100 p-4 font-mono text-xs leading-relaxed text-gray-800 dark:bg-gray-900 dark:text-gray-200">{`agent:
  enable_agent_teams: true
  profiles:
    fast:
      command: "claude --model claude-haiku-4-5-20251001"
      prompt: "Fast executor for simple, well-scoped tasks."
    researcher:
      command: "claude --model claude-opus-4-6"
      prompt: "Deep research and analysis agent."`}</pre>
            <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              Changes are hot-reloaded without restarting. Set{' '}
              <code className="rounded bg-gray-100 px-1 font-mono text-xs dark:bg-gray-800">
                agent_mode: teams
              </code>{' '}
              in WORKFLOW.md to enable Agent Teams mode.
            </p>
          </div>
        </div>
      </div>
    </>
  );
}
