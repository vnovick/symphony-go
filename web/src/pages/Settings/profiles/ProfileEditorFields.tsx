import {
  isSimpleBackendCommand,
  modelDatalistId,
  modelsForBackend,
  normalizeBackend,
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

interface ProfileEditorFieldsProps {
  backend: SupportedBackend;
  model: string;
  command: string;
  prompt: string;
  onBackendChange: (value: SupportedBackend) => void;
  onModelChange: (value: string) => void;
  onCommandChange: (value: string) => void;
  onPromptChange: (value: string) => void;
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
}: ProfileEditorFieldsProps) {
  const isCustomCommand = !isSimpleBackendCommand(command, backend);
  return (
    <>
      <BackendSelect value={backend} onChange={onBackendChange} />
      <ModelInput backend={backend} value={model} onChange={onModelChange} />
      <CommandInput value={command} backend={backend} onChange={onCommandChange} />
      {isCustomCommand && (
        <p className="text-[10px] text-theme-muted">
          Custom command detected — model selection may not apply.
        </p>
      )}
      <textarea
        value={prompt}
        onChange={(e) => {
          onPromptChange(e.target.value);
        }}
        placeholder="System prompt (optional) — appended to the workflow prompt."
        className={textareaCls}
        rows={3}
      />
    </>
  );
}
