import { useState, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Modal } from '../../components/ui/modal';
import type { ProfileDef } from '../../types/symphony';
import {
  applyBackendSelection,
  applyModelSelection,
  buildCanonicalCommand,
  commandToBackend,
  commandToModel,
  draftFromProfileDef,
  inferBackendFromCommand,
  isSimpleBackendCommand,
  modelDatalistId,
  modelLabel,
  modelsForBackend,
  normalizeBackend,
  normalizeCommandForSave,
  type SupportedBackend,
} from './profileCommands';
import { proseClass } from '../../utils/format';

// ─── Zod schemas ──────────────────────────────────────────────────────────────

const addProfileSchema = z.object({
  name: z
    .string()
    .min(1, 'Profile name is required.')
    .regex(/^\S+$/, 'Profile name must not contain spaces.'),
  backend: z.enum(['claude', 'codex']),
  model: z.string(),
  command: z.string().min(1, 'Command is required.'),
  prompt: z.string(),
});

const editProfileSchema = z.object({
  backend: z.enum(['claude', 'codex']),
  model: z.string(),
  command: z.string().min(1, 'Command is required.'),
  prompt: z.string(),
});

type AddProfileValues = z.infer<typeof addProfileSchema>;
type EditProfileValues = z.infer<typeof editProfileSchema>;

// ─── Suggested profiles ───────────────────────────────────────────────────────

const SUGGESTED_PROFILES = [
  {
    id: 'pm',
    label: 'Product Manager',
    description:
      'Clarifies requirements, writes acceptance criteria, and ensures work is unambiguous before development begins.',
    backend: 'claude' as SupportedBackend,
    model: 'claude-sonnet-4-6',
    prompt: `You are a **Product Manager specialist** embedded in a software development workflow. Your primary responsibility is ensuring every issue is clear, actionable, and testable — before development starts and after it finishes.

## When scoping an issue

- **Review the description critically.** Identify vague requirements, unstated assumptions, and missing context. If the "why" behind a feature is unclear, surface it before proceeding.
- **Write acceptance criteria** as a numbered checklist. Each criterion must be independently verifiable: _"The user can export data as CSV with a max of 10,000 rows"_, not _"Improve data export"_.
- **Decompose large issues** into focused sub-tasks, each completable in a single working session. Flag any issue that cannot be estimated without further clarification.
- **Define the definition of done.** State explicitly what "complete" means, including edge cases, error states, and non-functional requirements (performance targets, accessibility, security constraints).

## When reviewing completed work

- Verify **each acceptance criterion** is demonstrably met, not just implied by the implementation.
- Write a **stakeholder summary** — 3–5 sentences describing what was delivered and why it matters. No technical jargon.
- Flag **scope drift** (work done outside the original spec) and **deferred items** that require follow-up issues.

## Constraints

Do not write implementation code. Your output is specifications, acceptance criteria, and structured feedback. Raise blocking concerns as numbered questions, not vague objections. Assume the developer is competent — focus on clarity of intent, not how things are built.`,
  },
  {
    id: 'reviewer',
    label: 'Code Reviewer',
    description:
      'Systematic code reviews covering correctness, security, performance, and test quality — with prioritised findings.',
    backend: 'claude' as SupportedBackend,
    model: 'claude-opus-4-6',
    prompt: `You are a **Code Reviewer specialist** responsible for thorough, constructive reviews that improve correctness, security, and long-term maintainability.

## Review process

Work through each change systematically. For every finding, state: the file and location, the problem, and a concrete suggested fix or alternative approach.

Classify every finding with a severity prefix:
- **[CRITICAL]** — Must fix before merging. Introduces bugs, security vulnerabilities, or breaks existing contracts.
- **[MAJOR]** — Should fix. Significantly impacts reliability, performance, or maintainability.
- **[MINOR]** — Recommended improvement. Cleaner, safer, or more idiomatic.
- **[NIT]** — Style or preference. Fix only if trivial.

## What to examine

**Correctness**
- Logic errors, off-by-one mistakes, incorrect state transitions
- Race conditions and improper concurrent access
- Incorrect error propagation or silently swallowed exceptions

**Security**
- Injection vulnerabilities (SQL, command, template, path traversal)
- Improper input validation or missing output encoding
- Authentication and authorisation gaps
- Secrets, credentials, or PII in code, logs, or error messages

**Performance**
- N+1 queries and missing database indexes
- Unnecessary allocations or copies in hot paths
- Unbounded loops or missing pagination on large datasets
- Missing caching for expensive, repeated operations

**Test quality**
- Are happy paths, edge cases, and failure modes all covered?
- Do tests assert observable behaviour, or just implementation details?
- Are mocks hiding real integration problems?

## Constraints

Do not approve changes that contain CRITICAL findings. Be specific — _"this could be improved"_ is not a useful review comment. For every problem raised, provide an actionable path forward.`,
  },
  {
    id: 'qa',
    label: 'QA Engineer',
    description:
      'Designs test plans, enumerates edge cases, writes automated tests, and validates acceptance criteria against the implementation.',
    backend: 'claude' as SupportedBackend,
    model: 'claude-sonnet-4-6',
    prompt: `You are a **QA Engineer specialist** responsible for designing test strategies, writing test cases, and validating that software meets its acceptance criteria completely and correctly.

## When assigned to an issue

**Write a test plan before implementation begins.** The plan must cover:
- **Unit tests** — individual functions, components, and utilities in isolation
- **Integration tests** — interactions between components, services, and data layers
- **End-to-end scenarios** — complete user journeys from input to observable output

**Enumerate failure scenarios** systematically:
- Empty inputs, null values, and absent optional fields
- Boundary values — minimum, maximum, and just over/under limits
- Invalid state transitions and out-of-order operations
- Network failures, timeouts, and partial or malformed responses
- Concurrent access and race conditions where applicable

## When writing test cases

Use a clear **Arrange / Act / Assert** structure. Each test case must include:
1. A human-readable scenario name that describes the behaviour under test
2. Pre-conditions and any required setup
3. The specific action or input
4. The exact expected outcome — not "it works" but a precise assertion

Write tests in the project's existing testing framework. Prefer integration-level tests over pure unit tests when the integration itself is the risky part.

## When reviewing completed work

- Execute the full test plan and document pass/fail for each case.
- Write **bug reports** for any failures: steps to reproduce, expected vs. actual behaviour, environment and version details.
- Assess **regression risk**: what existing functionality could this change affect, and are those paths covered?

## Constraints

A test that always passes regardless of the implementation is worthless — and actively harmful. Flag acceptance criteria that are untestable back to the Product Manager before writing tests for them.`,
  },
] as const;

type SuggestedProfile = (typeof SUGGESTED_PROFILES)[number];

// ─── Shared style constants ───────────────────────────────────────────────────

const selectCls =
  'w-full rounded-[var(--radius-sm)] border px-3 py-2 text-[13px] cursor-pointer focus:outline-none bg-[var(--panel-strong)] border-[var(--line)] text-[var(--text)]';
const textareaCls =
  'w-full rounded-[var(--radius-sm)] border px-3 py-2 text-xs font-mono focus:outline-none resize-y min-h-[56px] bg-[var(--panel-strong)] border-[var(--line)] text-[var(--text)]';

function backendLabel(backend: SupportedBackend): string {
  return backend === 'codex' ? 'Codex' : 'Claude';
}

function backendBadgeClass(backend: SupportedBackend): string {
  return backend === 'codex'
    ? 'bg-[var(--teal-soft)] text-[var(--teal)]'
    : 'bg-[var(--accent-soft)] text-[var(--accent-strong)]';
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function ModelInput({
  backend,
  value,
  onChange,
}: {
  backend: SupportedBackend;
  value: string;
  onChange: (v: string) => void;
}) {
  const models = modelsForBackend(backend);
  const datalistId = modelDatalistId(backend);
  const placeholder =
    backend === 'codex'
      ? 'Model ID (e.g. gpt-5.2-codex) or leave blank for default'
      : 'Model ID (e.g. claude-sonnet-4-6) or leave blank for default';
  return (
    <>
      <datalist id={datalistId}>
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.label}
          </option>
        ))}
      </datalist>
      <input
        list={datalistId}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        placeholder={placeholder}
        className={selectCls}
      />
    </>
  );
}

function BackendSelect({
  value,
  onChange,
}: {
  value: SupportedBackend;
  onChange: (value: SupportedBackend) => void;
}) {
  return (
    <select
      value={value}
      onChange={(e) => {
        onChange(normalizeBackend(e.target.value));
      }}
      className={selectCls}
    >
      <option value="claude">Claude</option>
      <option value="codex">Codex</option>
    </select>
  );
}

function CommandInput({
  value,
  backend,
  onChange,
}: {
  value: string;
  backend: SupportedBackend;
  onChange: (value: string) => void;
}) {
  return (
    <input
      value={value}
      onChange={(e) => {
        onChange(e.target.value);
      }}
      placeholder={
        backend === 'codex'
          ? 'codex, /path/to/codex, or a wrapper command'
          : 'claude, /path/to/claude, or a wrapper command'
      }
      className={`${selectCls} font-mono text-xs`}
    />
  );
}

function ProfileEditorFields({
  backend,
  model,
  command,
  prompt,
  onBackendChange,
  onModelChange,
  onCommandChange,
  onPromptChange,
}: {
  backend: SupportedBackend;
  model: string;
  command: string;
  prompt: string;
  onBackendChange: (value: SupportedBackend) => void;
  onModelChange: (value: string) => void;
  onCommandChange: (value: string) => void;
  onPromptChange: (value: string) => void;
}) {
  const isCustomCommand = !isSimpleBackendCommand(command, backend);
  return (
    <>
      <BackendSelect value={backend} onChange={onBackendChange} />
      <ModelInput backend={backend} value={model} onChange={onModelChange} />
      <CommandInput value={command} backend={backend} onChange={onCommandChange} />
      <p className="text-[11px]" style={{ color: 'var(--text-secondary)' }}>
        Raw command is preserved as-is. Set the backend explicitly when this profile uses a wrapper
        script.
      </p>
      {isCustomCommand && (
        <p className="text-[11px]" style={{ color: 'var(--warning)' }}>
          Custom command detected. Backend selects the runner, and model changes only rewrite the
          `--model` flag.
        </p>
      )}
      <p className="text-[11px] font-medium tracking-wider uppercase" style={{ color: 'var(--muted)' }}>
        Role description
      </p>
      <textarea
        className={textareaCls}
        placeholder="System prompt for this specialist — describes its role, strengths, and when the orchestrator should delegate to it (used in Agent Teams mode)"
        value={prompt}
        onChange={(e) => {
          onPromptChange(e.target.value);
        }}
      />
    </>
  );
}

interface ProfileRowProps {
  name: string;
  def: ProfileDef;
  onEdit: (name: string, def: ProfileDef) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
}

function ProfileRow({ name, def, onEdit, onDelete }: ProfileRowProps) {
  const initial = draftFromProfileDef(def);
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const {
    handleSubmit,
    watch,
    setValue,
    reset,
    formState: { isSubmitting, errors },
  } = useForm<EditProfileValues>({
    resolver: zodResolver(editProfileSchema),
    defaultValues: {
      backend: initial.backend,
      model: initial.model,
      command: initial.command,
      prompt: initial.prompt,
    },
  });

  const [backend, model, command, prompt] = watch(['backend', 'model', 'command', 'prompt']);

  const handleCancel = () => {
    reset(draftFromProfileDef(def));
    setEditing(false);
  };

  const onSubmit = handleSubmit(async (values) => {
    await onEdit(name, {
      command: normalizeCommandForSave(values.command, values.backend),
      backend: values.backend,
      prompt: values.prompt.trim() || undefined,
    });
    setEditing(false);
  });

  if (editing) {
    return (
      <tr style={{ borderBottom: '1px solid var(--line)', background: 'var(--bg-soft)' }}>
        <td className="px-4 py-3 align-top font-mono text-sm" style={{ color: 'var(--text)' }}>
          {name}
        </td>
        <td className="space-y-2 px-4 py-3">
          <ProfileEditorFields
            backend={backend}
            model={model}
            command={command}
            prompt={prompt}
            onBackendChange={(value) => {
              const next = applyBackendSelection(command, backend, value);
              setValue('backend', value);
              setValue('model', next.model);
              setValue('command', next.command);
            }}
            onModelChange={(value) => {
              setValue('model', value);
              setValue('command', applyModelSelection(command, backend, value));
            }}
            onCommandChange={(value) => {
              setValue('command', value);
              setValue('model', commandToModel(value));
              const inferred = inferBackendFromCommand(value);
              if (inferred) setValue('backend', inferred);
            }}
            onPromptChange={(value) => {
              setValue('prompt', value);
            }}
          />
          {errors.command && (
            <p role="alert" className="text-xs" style={{ color: 'var(--danger)' }}>
              {errors.command.message}
            </p>
          )}
        </td>
        <td className="px-4 py-3 text-right align-top whitespace-nowrap">
          <button
            onClick={() => {
              void onSubmit();
            }}
            disabled={isSubmitting}
            className="mr-2 rounded-[var(--radius-sm)] px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
            style={{ background: 'var(--accent)' }}
          >
            {isSubmitting ? 'Saving…' : 'Save'}
          </button>
          <button
            onClick={handleCancel}
            className="rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
            style={{ borderColor: 'var(--line)', color: 'var(--text-secondary)' }}
          >
            Cancel
          </button>
        </td>
      </tr>
    );
  }

  const displayBackend = commandToBackend(def.command, def.backend);
  const modelId = commandToModel(def.command);
  const displayModel = modelId ? modelLabel(displayBackend, modelId) : 'Default';
  const displayCommand = normalizeCommandForSave(def.command, displayBackend);
  return (
    <tr
      className="transition-colors hover:bg-[var(--bg-soft)]"
      style={{ borderBottom: '1px solid var(--line)' }}
    >
      <td className="px-4 py-3 align-top font-mono text-sm font-bold" style={{ color: 'var(--text)' }}>
        {name}
      </td>
      <td className="space-y-1 px-4 py-3 align-top">
        <span
          className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ${backendBadgeClass(backend)}`}
        >
          {backendLabel(backend)}
        </span>
        <span
          className="ml-2 inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium"
          style={{ background: 'var(--bg-soft)', color: 'var(--text-secondary)' }}
        >
          {displayModel}
        </span>
        {def.prompt && (
          <p
            className="max-w-xs truncate text-xs italic"
            style={{ color: 'var(--muted)' }}
            title={def.prompt}
          >
            {def.prompt.slice(0, 80)}
            {def.prompt.length > 80 ? '…' : ''}
          </p>
        )}
        <p
          className="max-w-xs truncate font-mono text-[11px]"
          style={{ color: 'var(--text-secondary)' }}
          title={displayCommand}
        >
          {displayCommand}
        </p>
      </td>
      <td className="px-4 py-3 text-right align-top whitespace-nowrap">
        {confirmDelete ? (
          <span className="inline-flex items-center gap-2">
            <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              Delete "{name}"?
            </span>
            <button
              onClick={async () => {
                setDeleting(true);
                await onDelete(name);
                setDeleting(false);
                setConfirmDelete(false);
              }}
              disabled={deleting}
              className="rounded-[var(--radius-sm)] px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
              style={{ background: 'var(--danger)' }}
            >
              {deleting ? 'Deleting…' : 'Confirm'}
            </button>
            <button
              onClick={() => { setConfirmDelete(false); }}
              disabled={deleting}
              className="rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
              style={{ borderColor: 'var(--line)', color: 'var(--text-secondary)' }}
            >
              Cancel
            </button>
          </span>
        ) : (
          <>
            <button
              onClick={() => {
                reset(draftFromProfileDef(def));
                setEditing(true);
              }}
              className="mr-2 rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
              style={{ borderColor: 'var(--line)', color: 'var(--text-secondary)' }}
            >
              Edit
            </button>
            <button
              onClick={() => { setConfirmDelete(true); }}
              className="rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
              style={{ borderColor: 'var(--danger-soft)', color: 'var(--danger)' }}
            >
              Delete
            </button>
          </>
        )}
      </td>
    </tr>
  );
}

function SuggestedProfileCard({
  suggestion,
  onAdd,
  onPreview,
  saving,
}: {
  suggestion: SuggestedProfile;
  onAdd: (s: SuggestedProfile) => Promise<void>;
  onPreview: (s: SuggestedProfile) => void;
  saving: boolean;
}) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => {
        onPreview(suggestion);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') onPreview(suggestion);
      }}
      className="flex cursor-pointer flex-col gap-2 rounded-[var(--radius-md)] border border-dashed p-3 transition-all hover:opacity-90"
      style={{ borderColor: 'var(--line)', background: 'var(--bg-soft)' }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-xs font-semibold" style={{ color: 'var(--text)' }}>
            {suggestion.label}
          </p>
          <span
            className={`mt-0.5 inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium ${backendBadgeClass(suggestion.backend)}`}
          >
            {backendLabel(suggestion.backend)} · {suggestion.model}
          </span>
        </div>
        <button
          onClick={(e) => {
            e.stopPropagation();
            void onAdd(suggestion);
          }}
          disabled={saving}
          className="flex-shrink-0 rounded-[var(--radius-sm)] border px-2 py-1 text-xs font-medium transition-colors disabled:opacity-50 hover:opacity-80"
          style={{ borderColor: 'var(--line)', background: 'var(--panel)', color: 'var(--text-secondary)' }}
        >
          {saving ? '…' : '+ Add'}
        </button>
      </div>
      <p className="text-[11px] leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
        {suggestion.description}
      </p>
      <p className="text-[10px]" style={{ color: 'var(--muted)' }}>Click to preview full prompt</p>
    </div>
  );
}

function TemplatePreviewModal({
  suggestion,
  onClose,
  onAdd,
  saving,
}: {
  suggestion: SuggestedProfile | null;
  onClose: () => void;
  onAdd: (s: SuggestedProfile) => Promise<void>;
  saving: boolean;
}) {
  return (
    <Modal isOpen={suggestion !== null} onClose={onClose} className="mx-4 my-8 max-w-2xl">
      {suggestion && (
        <div className="space-y-5 p-6">
          <div>
            <h2 className="text-lg font-semibold" style={{ color: 'var(--text)' }}>
              {suggestion.label}
            </h2>
            <p className="mt-0.5 text-sm" style={{ color: 'var(--text-secondary)' }}>
              {suggestion.description}
            </p>
            <div className="mt-2 flex items-center gap-2">
              <span
                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${backendBadgeClass(suggestion.backend)}`}
              >
                {backendLabel(suggestion.backend)}
              </span>
              <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
                {suggestion.model}
              </span>
              <span className="text-xs" style={{ color: 'var(--muted)' }}>
                · profile id:{' '}
                <code
                  className="rounded px-1 font-mono text-[11px]"
                  style={{ background: 'var(--bg-soft)', color: 'var(--text-secondary)' }}
                >
                  {suggestion.id}
                </code>
              </span>
            </div>
          </div>
          <div
            className="rounded-[var(--radius-md)] p-4"
            style={{ border: '1px solid var(--line)', background: 'var(--panel-strong)' }}
          >
            <p
              className="mb-3 text-[11px] font-medium tracking-wider uppercase"
              style={{ color: 'var(--muted)' }}
            >
              Role description — system prompt saved to WORKFLOW.md
            </p>
            <div className={proseClass}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{suggestion.prompt}</ReactMarkdown>
            </div>
          </div>
          <div
            className="flex items-center justify-end gap-2 border-t pt-4"
            style={{ borderColor: 'var(--line)' }}
          >
            <button
              onClick={onClose}
              className="rounded-[var(--radius-sm)] border px-4 py-2 text-sm transition-colors hover:opacity-80"
              style={{ borderColor: 'var(--line)', color: 'var(--text-secondary)' }}
            >
              Cancel
            </button>
            <button
              onClick={() => {
                void onAdd(suggestion);
                onClose();
              }}
              disabled={saving}
              className="rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
              style={{ background: 'var(--accent)' }}
            >
              {saving ? 'Adding…' : `Add "${suggestion.id}" profile`}
            </button>
          </div>
        </div>
      )}
    </Modal>
  );
}

// ─── ProfilesCard ─────────────────────────────────────────────────────────────

interface ProfilesCardProps {
  profileDefs: Record<string, ProfileDef>;
  onUpsert: (name: string, command: string, backend?: string, prompt?: string) => Promise<boolean>;
  onDelete: (name: string) => Promise<boolean>;
}

export function ProfilesCard({ profileDefs, onUpsert, onDelete }: ProfilesCardProps) {
  const [uiState, setUiState] = useState({ adding: false, deleteError: '' });
  const { adding, deleteError } = uiState;
  const [quickAddSaving, setQuickAddSaving] = useState<string | null>(null);
  const [previewSuggestion, setPreviewSuggestion] = useState<SuggestedProfile | null>(null);

  const addForm = useForm<AddProfileValues>({
    resolver: zodResolver(addProfileSchema),
    defaultValues: {
      name: '',
      backend: 'claude',
      model: '',
      command: buildCanonicalCommand('claude', ''),
      prompt: '',
    },
  });
  const [addBackend, addModel, addCommand, addPrompt] = addForm.watch([
    'backend',
    'model',
    'command',
    'prompt',
  ]);

  const profileEntries = useMemo(
    () => Object.entries(profileDefs).sort(([a], [b]) => a.localeCompare(b)),
    [profileDefs],
  );
  const suggestedToShow = useMemo(
    () => SUGGESTED_PROFILES.filter((s) => !(s.id in profileDefs)),
    [profileDefs],
  );

  const openAddForm = () => {
    addForm.reset({
      name: '',
      backend: 'claude',
      model: '',
      command: buildCanonicalCommand('claude', ''),
      prompt: '',
    });
    setUiState((s) => ({ ...s, adding: true }));
  };

  const handleAddCancel = () => {
    addForm.reset();
    setUiState((s) => ({ ...s, adding: false }));
  };

  const handleEdit = async (name: string, def: ProfileDef) => {
    await onUpsert(
      name,
      normalizeCommandForSave(def.command, commandToBackend(def.command, def.backend)),
      def.backend,
      def.prompt,
    );
  };

  const handleDelete = async (name: string) => {
    setUiState((s) => ({ ...s, deleteError: '' }));
    const ok = await onDelete(name);
    if (!ok)
      setUiState((s) => ({
        ...s,
        deleteError: `Failed to delete profile "${name}". Check the server logs.`,
      }));
  };

  const handleAdd = addForm.handleSubmit(async (values) => {
    const ok = await onUpsert(
      values.name.trim(),
      normalizeCommandForSave(values.command, values.backend),
      values.backend,
      values.prompt.trim() || undefined,
    );
    if (ok) {
      addForm.reset();
      setUiState((s) => ({ ...s, adding: false }));
    } else {
      addForm.setError('root', { message: 'Failed to save profile. Check the server logs.' });
    }
  });

  const handleQuickAdd = async (suggestion: SuggestedProfile) => {
    setQuickAddSaving(suggestion.id);
    await onUpsert(
      suggestion.id,
      buildCanonicalCommand(suggestion.backend, suggestion.model),
      suggestion.backend,
      suggestion.prompt,
    );
    setQuickAddSaving(null);
  };

  return (
    <>
      <div
        className="overflow-hidden rounded-[var(--radius-md)]"
        style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)' }}
      >
        <div
          className="flex items-center justify-between border-b px-5 py-4"
          style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)' }}
        >
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text)' }}>
              Agent Profiles
            </h2>
            <p className="mt-0.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
              Select per-issue from the issue detail modal. Backend and model controls stay
              backend-aware, and custom wrapper commands are preserved instead of flattened.
            </p>
          </div>
          {!adding && (
            <button
              onClick={openAddForm}
              className="flex items-center gap-1.5 rounded-[var(--radius-sm)] px-3 py-1.5 text-xs font-medium text-white transition-colors hover:opacity-90"
              style={{ background: 'var(--accent)' }}
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
            <thead style={{ background: 'var(--bg-soft)' }}>
              <tr>
                <th
                  className="w-40 px-4 py-3 text-left text-xs font-medium tracking-wider uppercase"
                  style={{ color: 'var(--muted)' }}
                >
                  Name
                </th>
                <th
                  className="px-4 py-3 text-left text-xs font-medium tracking-wider uppercase"
                  style={{ color: 'var(--muted)' }}
                >
                  Backend / Model
                </th>
                <th className="w-40 px-4 py-3" />
              </tr>
            </thead>
            <tbody>
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
                <tr style={{ borderBottom: '1px solid var(--line)', background: 'var(--bg-soft)' }}>
                  <td className="px-4 py-3 align-top">
                    <input
                      className="w-full rounded border px-3 py-1.5 font-mono text-sm focus:outline-none focus:ring-1"
                      style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)', color: 'var(--text)' }}
                      placeholder="profile-name"
                      {...addForm.register('name')}
                      onKeyDown={(e) => {
                        if (e.key === 'Escape') handleAddCancel();
                      }}
                      autoFocus
                    />
                    {addForm.formState.errors.name && (
                      <p role="alert" className="mt-1 text-xs" style={{ color: 'var(--danger)' }}>
                        {addForm.formState.errors.name.message}
                      </p>
                    )}
                  </td>
                  <td className="space-y-2 px-4 py-3">
                    <ProfileEditorFields
                      backend={addBackend}
                      model={addModel}
                      command={addCommand}
                      prompt={addPrompt}
                      onBackendChange={(value) => {
                        const next = applyBackendSelection(addCommand, addBackend, value);
                        addForm.setValue('backend', value);
                        addForm.setValue('model', next.model);
                        addForm.setValue('command', next.command);
                      }}
                      onModelChange={(value) => {
                        addForm.setValue('model', value);
                        addForm.setValue(
                          'command',
                          applyModelSelection(addCommand, addBackend, value),
                        );
                      }}
                      onCommandChange={(value) => {
                        addForm.setValue('command', value);
                        addForm.setValue('model', commandToModel(value));
                        const inferred = inferBackendFromCommand(value);
                        if (inferred) addForm.setValue('backend', inferred);
                      }}
                      onPromptChange={(value) => {
                        addForm.setValue('prompt', value);
                      }}
                    />
                    {addForm.formState.errors.command && (
                      <p role="alert" className="text-xs" style={{ color: 'var(--danger)' }}>
                        {addForm.formState.errors.command.message}
                      </p>
                    )}
                    {addForm.formState.errors.root && (
                      <p role="alert" className="text-xs" style={{ color: 'var(--danger)' }}>
                        {addForm.formState.errors.root.message}
                      </p>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right align-top whitespace-nowrap">
                    <button
                      onClick={() => {
                        void handleAdd();
                      }}
                      disabled={addForm.formState.isSubmitting}
                      className="mr-2 rounded-[var(--radius-sm)] px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
                      style={{ background: 'var(--accent)' }}
                    >
                      {addForm.formState.isSubmitting ? 'Saving…' : 'Save'}
                    </button>
                    <button
                      onClick={handleAddCancel}
                      className="rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
                      style={{ borderColor: 'var(--line)', color: 'var(--text-secondary)' }}
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
                    className="px-4 py-10 text-center text-sm"
                    style={{ color: 'var(--muted)' }}
                  >
                    No profiles configured yet.{' '}
                    <button
                      onClick={openAddForm}
                      className="hover:underline"
                      style={{ color: 'var(--accent)' }}
                    >
                      Add one
                    </button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {suggestedToShow.length > 0 && (
          <div className="border-t px-6 py-4" style={{ borderColor: 'var(--line)' }}>
            <p className="mb-3 text-[11px] font-medium tracking-wider uppercase" style={{ color: 'var(--muted)' }}>
              Quick-add templates
            </p>
            <div className="grid grid-cols-3 gap-3">
              {suggestedToShow.map((s) => (
                <SuggestedProfileCard
                  key={s.id}
                  suggestion={s}
                  onAdd={handleQuickAdd}
                  onPreview={setPreviewSuggestion}
                  saving={quickAddSaving === s.id}
                />
              ))}
            </div>
          </div>
        )}
      </div>

      {deleteError && <p className="text-sm" style={{ color: 'var(--danger)' }}>{deleteError}</p>}

      <TemplatePreviewModal
        suggestion={previewSuggestion}
        onClose={() => {
          setPreviewSuggestion(null);
        }}
        onAdd={handleQuickAdd}
        saving={previewSuggestion !== null && quickAddSaving === previewSuggestion.id}
      />
    </>
  );
}
