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
  'w-full bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded px-3 py-1.5 text-sm text-gray-800 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-brand-500';
const textareaCls =
  'w-full bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded px-3 py-1.5 text-xs text-gray-800 dark:text-gray-100 font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 resize-y min-h-[56px]';
const proseClass = [
  'prose prose-sm dark:prose-invert max-w-none',
  'text-gray-800 dark:text-gray-200',
  'prose-p:my-1 prose-p:leading-relaxed',
  'prose-headings:font-semibold prose-headings:mt-4 prose-headings:mb-1',
  'prose-code:text-xs prose-code:bg-gray-100 dark:prose-code:bg-gray-800 prose-code:px-1 prose-code:rounded prose-code:before:content-none prose-code:after:content-none',
  'prose-ul:my-1 prose-ol:my-1 prose-li:my-0.5',
  'prose-strong:text-gray-900 dark:prose-strong:text-white',
  'prose-em:text-gray-600 dark:prose-em:text-gray-300',
].join(' ');

function backendLabel(backend: SupportedBackend): string {
  return backend === 'codex' ? 'Codex' : 'Claude';
}

function backendBadgeClass(backend: SupportedBackend): string {
  return backend === 'codex'
    ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
    : 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300';
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
      <p className="text-[11px] text-gray-500 dark:text-gray-400">
        Raw command is preserved as-is. Set the backend explicitly when this profile uses a wrapper
        script.
      </p>
      {isCustomCommand && (
        <p className="text-[11px] text-amber-600 dark:text-amber-400">
          Custom command detected. Backend selects the runner, and model changes only rewrite the
          `--model` flag.
        </p>
      )}
      <p className="text-[11px] font-medium tracking-wider text-gray-500 uppercase dark:text-gray-400">
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
      <tr className="border-b border-gray-200 bg-gray-50/50 dark:border-gray-700 dark:bg-gray-800/20">
        <td className="px-4 py-3 align-top font-mono text-sm text-gray-700 dark:text-gray-200">
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
            <p role="alert" className="text-xs text-red-500 dark:text-red-400">
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
            className="bg-brand-500 hover:bg-brand-600 mr-2 rounded-lg px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
          >
            {isSubmitting ? 'Saving…' : 'Save'}
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

  const displayBackend = commandToBackend(def.command, def.backend);
  const modelId = commandToModel(def.command);
  const displayModel = modelId ? modelLabel(displayBackend, modelId) : 'Default';
  const displayCommand = normalizeCommandForSave(def.command, displayBackend);
  return (
    <tr className="border-b border-gray-200 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800/50">
      <td className="px-4 py-3 align-top font-mono text-sm text-gray-700 dark:text-gray-200">
        {name}
      </td>
      <td className="space-y-1 px-4 py-3 align-top">
        <span
          className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ${backendBadgeClass(backend)}`}
        >
          {backendLabel(backend)}
        </span>
        <span className="ml-2 inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-gray-800 dark:text-gray-300">
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
        <p
          className="max-w-xs truncate font-mono text-[11px] text-gray-500 dark:text-gray-400"
          title={displayCommand}
        >
          {displayCommand}
        </p>
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
                reset(draftFromProfileDef(def));
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
      className="flex cursor-pointer flex-col gap-2 rounded-xl border border-dashed border-gray-200 bg-gray-50/50 p-3 transition-colors hover:border-gray-300 hover:bg-gray-100/60 dark:border-gray-700 dark:bg-gray-800/20 dark:hover:border-gray-600 dark:hover:bg-gray-800/40"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-xs font-semibold text-gray-700 dark:text-gray-200">
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
          className="flex-shrink-0 rounded-lg border border-gray-300 bg-white px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
        >
          {saving ? '…' : '+ Add'}
        </button>
      </div>
      <p className="text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
        {suggestion.description}
      </p>
      <p className="text-[10px] text-gray-400 dark:text-gray-600">Click to preview full prompt</p>
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
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
              {suggestion.label}
            </h2>
            <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
              {suggestion.description}
            </p>
            <div className="mt-2 flex items-center gap-2">
              <span
                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${backendBadgeClass(suggestion.backend)}`}
              >
                {backendLabel(suggestion.backend)}
              </span>
              <span className="font-mono text-xs text-gray-500 dark:text-gray-400">
                {suggestion.model}
              </span>
              <span className="text-xs text-gray-400 dark:text-gray-500">
                · profile id:{' '}
                <code className="rounded bg-gray-100 px-1 font-mono text-[11px] dark:bg-gray-800">
                  {suggestion.id}
                </code>
              </span>
            </div>
          </div>
          <div className="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-900/50">
            <p className="mb-3 text-[11px] font-medium tracking-wider text-gray-400 uppercase dark:text-gray-500">
              Role description — system prompt saved to WORKFLOW.md
            </p>
            <div className={proseClass}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{suggestion.prompt}</ReactMarkdown>
            </div>
          </div>
          <div className="flex items-center justify-end gap-2 border-t border-gray-100 pt-4 dark:border-gray-800">
            <button
              onClick={onClose}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
            <button
              onClick={() => {
                void onAdd(suggestion);
                onClose();
              }}
              disabled={saving}
              className="bg-brand-500 hover:bg-brand-600 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
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
      <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
        <div className="flex items-center justify-between border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
          <div>
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
              Agent Profiles
            </h2>
            <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              Select per-issue from the issue detail modal. Backend and model controls stay
              backend-aware, and custom wrapper commands are preserved instead of flattened.
            </p>
          </div>
          {!adding && (
            <button
              onClick={openAddForm}
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
                  Backend / Model
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
                      {...addForm.register('name')}
                      onKeyDown={(e) => {
                        if (e.key === 'Escape') handleAddCancel();
                      }}
                      autoFocus
                    />
                    {addForm.formState.errors.name && (
                      <p role="alert" className="mt-1 text-xs text-red-500 dark:text-red-400">
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
                      <p role="alert" className="text-xs text-red-500 dark:text-red-400">
                        {addForm.formState.errors.command.message}
                      </p>
                    )}
                    {addForm.formState.errors.root && (
                      <p role="alert" className="text-xs text-red-500 dark:text-red-400">
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
                      className="bg-brand-500 hover:bg-brand-600 mr-2 rounded-lg px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
                    >
                      {addForm.formState.isSubmitting ? 'Saving…' : 'Save'}
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
                    <button onClick={openAddForm} className="text-brand-500 hover:underline">
                      Add one
                    </button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {suggestedToShow.length > 0 && (
          <div className="border-t border-gray-100 px-6 py-4 dark:border-gray-800">
            <p className="mb-3 text-[11px] font-medium tracking-wider text-gray-400 uppercase dark:text-gray-500">
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

      {deleteError && <p className="text-sm text-red-600 dark:text-red-400">{deleteError}</p>}

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
