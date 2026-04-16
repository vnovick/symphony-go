import { useState } from 'react';
import type { ProfileDef } from '../../../types/schemas';
import {
  AGENT_ACTION_OPTIONS,
  commandToBackend,
  commandToModel,
  modelLabel,
  normalizeAllowedActions,
} from '../profileCommands';
import { backendBadgeClass, backendLabel } from './ProfileEditorFields';

interface ProfileRowProps {
  name: string;
  def: ProfileDef;
  onEdit: () => void;
  onToggleEnabled: (name: string, def: ProfileDef, enabled: boolean) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
}

export function ProfileRow({ name, def, onEdit, onToggleEnabled, onDelete }: ProfileRowProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [pendingAction, setPendingAction] = useState<'toggle' | 'delete' | null>(null);

  const inferredBackend = commandToBackend(def.command, def.backend);
  const inferredModel = commandToModel(def.command);
  const actionLabels = AGENT_ACTION_OPTIONS.filter((option) =>
    normalizeAllowedActions(def.allowedActions).includes(option.id),
  ).map((option) => option.label);
  const isEnabled = def.enabled ?? true;

  return (
    <article className="border-theme-line bg-theme-bg-soft flex min-h-[176px] w-full flex-col gap-3 rounded-[var(--radius-md)] border p-4">
      <div className="flex w-full items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-theme-text text-sm font-semibold">{name}</p>
            <span
              className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium ${backendBadgeClass(inferredBackend)}`}
            >
              {backendLabel(inferredBackend)}
            </span>
            {inferredModel && (
              <span className="bg-theme-panel text-theme-text-secondary rounded-full px-2 py-0.5 font-mono text-[10px]">
                {modelLabel(inferredBackend, inferredModel)}
              </span>
            )}
          </div>
          <p className="text-theme-text-secondary mt-2 text-[11px] leading-relaxed">
            {def.prompt?.trim()
              ? `${def.prompt.slice(0, 180)}${def.prompt.length > 180 ? '…' : ''}`
              : 'No profile prompt configured yet.'}
          </p>
        </div>
        <span
          className={`rounded-[var(--radius-sm)] px-2.5 py-1 text-[11px] font-medium whitespace-nowrap ${
            isEnabled
              ? 'bg-theme-success-soft text-theme-success'
              : 'bg-theme-panel text-theme-text-secondary'
          }`}
        >
          {isEnabled ? 'Active' : 'Inactive'}
        </span>
      </div>

      {actionLabels.length > 0 && (
        <div className="mt-auto flex flex-wrap gap-1">
          {actionLabels.map((label) => (
            <span
              key={label}
              className="bg-theme-panel text-theme-text-secondary rounded-full px-2 py-0.5 text-[10px]"
            >
              {label}
            </span>
          ))}
        </div>
      )}

      <div className="mt-auto flex flex-wrap items-center gap-2 pt-1">
        <button
          onClick={onEdit}
          className="border-theme-line text-theme-text-secondary rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs transition-colors hover:opacity-80"
        >
          Edit
        </button>
        <button
          onClick={async () => {
            setPendingAction('toggle');
            await onToggleEnabled(name, def, !isEnabled);
            setPendingAction(null);
          }}
          disabled={pendingAction !== null}
          className="bg-theme-accent rounded-[var(--radius-sm)] px-3 py-1.5 text-xs font-medium text-white transition-colors disabled:opacity-50"
        >
          {pendingAction === 'toggle' ? 'Saving…' : isEnabled ? 'Deactivate' : 'Activate'}
        </button>
        {confirmDelete ? (
          <>
            <span className="text-theme-muted text-xs">Delete?</span>
            <button
              onClick={async () => {
                setPendingAction('delete');
                await onDelete(name);
                setPendingAction(null);
                setConfirmDelete(false);
              }}
              disabled={pendingAction !== null}
              className="bg-theme-danger rounded-[var(--radius-sm)] px-2.5 py-1.5 text-xs font-medium text-white transition-colors disabled:opacity-50"
            >
              {pendingAction === 'delete' ? '…' : 'Yes'}
            </button>
            <button
              onClick={() => {
                setConfirmDelete(false);
              }}
              disabled={pendingAction !== null}
              className="border-theme-line text-theme-text-secondary rounded-[var(--radius-sm)] border px-2.5 py-1.5 text-xs transition-colors hover:opacity-80 disabled:opacity-50"
            >
              No
            </button>
          </>
        ) : (
          <button
            onClick={() => {
              setConfirmDelete(true);
            }}
            disabled={pendingAction !== null}
            className="border-theme-danger text-theme-danger rounded-[var(--radius-sm)] border px-3 py-1.5 text-xs transition-colors hover:opacity-80 disabled:opacity-50"
          >
            Delete
          </button>
        )}
      </div>
    </article>
  );
}
