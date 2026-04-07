import {
  isSimpleBackendCommand,
  modelsForBackend,
  normalizeBackend,
  type ModelOption,
  type SupportedBackend,
} from '../profileCommands';

export const selectCls =
  'w-full rounded-[var(--radius-sm)] border px-3 py-2 text-[13px] cursor-pointer focus:outline-none bg-[var(--panel-strong)] border-[var(--line)] text-[var(--text)]';

export const textareaCls =
  'w-full rounded-[var(--radius-sm)] border px-3 py-2 text-xs font-mono focus:outline-none resize-y min-h-[56px] bg-[var(--panel-strong)] border-[var(--line)] text-[var(--text)]';

export function backendLabel(backend: SupportedBackend): string {
  return backend === 'codex' ? 'Codex' : 'Claude';
}

export function backendBadgeClass(backend: SupportedBackend): string {
  return backend === 'codex'
    ? 'bg-[var(--teal-soft)] text-[var(--teal)]'
    : 'bg-[var(--accent-soft)] text-[var(--accent-strong)]';
}

function ModelInput({
  backend,
  value,
  onChange,
  dynamicModels,
}: {
  backend: SupportedBackend;
  value: string;
  onChange: (v: string) => void;
  dynamicModels?: Record<string, ModelOption[]>;
}) {
  const models = modelsForBackend(backend, dynamicModels);
  const isKnownModel = !value || models.some((m) => m.id === value);
  return (
    <>
      <select
        value={isKnownModel ? value : '__custom__'}
        onChange={(e) => {
          const v = e.target.value;
          onChange(v === '__custom__' ? '' : v);
        }}
        className={selectCls}
      >
        <option value="">Default model</option>
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.id} — {m.label}
          </option>
        ))}
        <option value="__custom__">Custom model ID…</option>
      </select>
      {!isKnownModel && (
        <input
          value={value}
          onChange={(e) => {
            onChange(e.target.value);
          }}
          placeholder="Enter custom model ID"
          className={`${selectCls} mt-1 font-mono text-xs`}
        />
      )}
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

interface ProfileEditorFieldsProps {
  backend: SupportedBackend;
  model: string;
  command: string;
  prompt: string;
  onBackendChange: (value: SupportedBackend) => void;
  onModelChange: (value: string) => void;
  onCommandChange: (value: string) => void;
  onPromptChange: (value: string) => void;
  dynamicModels?: Record<string, ModelOption[]>;
}

export function ProfileEditorFields({
  backend,
  model,
  command,
  prompt,
  onBackendChange,
  onModelChange,
  onCommandChange,
  onPromptChange,
  dynamicModels,
}: ProfileEditorFieldsProps) {
  const isCustomCommand = !isSimpleBackendCommand(command, backend);
  return (
    <>
      <BackendSelect value={backend} onChange={onBackendChange} />
      <ModelInput
        backend={backend}
        value={model}
        onChange={onModelChange}
        dynamicModels={dynamicModels}
      />
      <CommandInput value={command} backend={backend} onChange={onCommandChange} />
      {isCustomCommand && (
        <p className="text-theme-muted text-[10px]">
          Custom command detected — model selection may not apply.
        </p>
      )}
      <textarea
        value={prompt}
        onChange={(e) => {
          onPromptChange(e.target.value);
        }}
        placeholder="System prompt (optional) — appended to the workflow prompt. Supports Liquid variables."
        className={textareaCls}
        rows={3}
      />
      <details className="text-theme-muted text-[10px]">
        <summary className="hover:text-theme-text-secondary cursor-pointer transition-colors">
          Available template variables
        </summary>
        <div className="border-theme-line mt-1 ml-1 space-y-0.5 border-l-2 pl-2 font-mono">
          <p>{'{{ issue.identifier }}'} — Issue ID (e.g. ENG-42)</p>
          <p>{'{{ issue.title }}'} — Issue title</p>
          <p>{'{{ issue.description }}'} — Issue body</p>
          <p>{'{{ issue.url }}'} — Issue URL</p>
          <p>{'{{ issue.branch_name }}'} — Git branch name</p>
          <p>{'{{ issue.labels }}'} — Labels array</p>
          <p>{'{{ issue.priority }}'} — Priority level</p>
          <p>{'{{ attempt }}'} — Retry attempt number</p>
        </div>
      </details>
    </>
  );
}
