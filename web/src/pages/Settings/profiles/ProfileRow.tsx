import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import type { ProfileDef } from '../../../types/schemas';
import {
  applyBackendSelection,
  applyModelSelection,
  commandToBackend,
  commandToModel,
  draftFromProfileDef,
  inferBackendFromCommand,
  modelLabel,
  normalizeCommandForSave,
} from '../profileCommands';
import { ProfileEditorFields, backendLabel, backendBadgeClass } from './ProfileEditorFields';

const editProfileSchema = z.object({
  backend: z.enum(['claude', 'codex']),
  model: z.string(),
  command: z.string().min(1, 'Command is required.'),
  prompt: z.string(),
});

type EditProfileValues = z.infer<typeof editProfileSchema>;

interface ProfileRowProps {
  name: string;
  def: ProfileDef;
  onEdit: (name: string, def: ProfileDef) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
  availableModels?: Record<string, { id: string; label: string }[]>;
}

export function ProfileRow({ name, def, onEdit, onDelete, availableModels }: ProfileRowProps) {
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
      <tr className="border-theme-line bg-theme-bg-soft border-b">
        <td className="text-theme-text px-4 py-3 align-top font-mono text-sm">{name}</td>
        <td className="space-y-2 px-4 py-3">
          <ProfileEditorFields
            backend={backend}
            model={model}
            command={command}
            prompt={prompt}
            onBackendChange={(value) => {
              const next = applyBackendSelection(command, backend, value);
              setValue('backend', value, { shouldValidate: true });
              setValue('model', next.model);
              setValue('command', next.command, { shouldValidate: true });
            }}
            onModelChange={(value) => {
              setValue('model', value);
              setValue('command', applyModelSelection(command, backend, value), {
                shouldValidate: true,
              });
            }}
            onCommandChange={(value) => {
              setValue('command', value, { shouldValidate: true });
              setValue('model', commandToModel(value));
              const inferred = inferBackendFromCommand(value);
              if (inferred) setValue('backend', inferred);
            }}
            onPromptChange={(value) => {
              setValue('prompt', value);
            }}
            dynamicModels={availableModels}
          />
          {errors.command && (
            <p role="alert" className="text-theme-danger text-xs">
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
            className="bg-theme-accent mr-2 rounded-[var(--radius-sm)] px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
          >
            {isSubmitting ? 'Saving…' : 'Save'}
          </button>
          <button
            onClick={handleCancel}
            className="border-theme-line text-theme-text-secondary rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
          >
            Cancel
          </button>
        </td>
      </tr>
    );
  }

  const inferredBackend = commandToBackend(def.command, def.backend);
  const inferredModel = commandToModel(def.command);

  return (
    <tr className="border-theme-line border-b">
      <td className="text-theme-text px-4 py-3 font-mono text-sm">{name}</td>
      <td className="px-4 py-3">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span
              className={`inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium ${backendBadgeClass(inferredBackend)}`}
            >
              {backendLabel(inferredBackend)}
            </span>
            {inferredModel && (
              <span className="text-theme-text-secondary font-mono text-xs">
                {modelLabel(inferredBackend, inferredModel)}
              </span>
            )}
          </div>
          {def.prompt && (
            <p className="max-w-[400px] truncate text-xs" title={def.prompt}>
              {def.prompt.slice(0, 120)}
            </p>
          )}
        </div>
      </td>
      <td className="px-4 py-3 text-right whitespace-nowrap">
        {confirmDelete ? (
          <>
            <span className="text-theme-muted mr-2 text-xs">Delete?</span>
            <button
              onClick={async () => {
                setDeleting(true);
                await onDelete(name);
                setDeleting(false);
                setConfirmDelete(false);
              }}
              disabled={deleting}
              className="bg-theme-danger mr-1 rounded-[var(--radius-sm)] px-2 py-1 text-xs font-medium text-white transition-colors disabled:opacity-50"
            >
              {deleting ? '…' : 'Yes'}
            </button>
            <button
              onClick={() => {
                setConfirmDelete(false);
              }}
              className="border-theme-line text-theme-text-secondary rounded-[var(--radius-sm)] border px-2 py-1 text-xs transition-colors hover:opacity-80"
            >
              No
            </button>
          </>
        ) : (
          <>
            <button
              onClick={() => {
                setEditing(true);
              }}
              className="border-theme-line text-theme-text-secondary mr-1 rounded-[var(--radius-sm)] border px-2 py-1 text-xs transition-colors hover:opacity-80"
            >
              Edit
            </button>
            <button
              onClick={() => {
                setConfirmDelete(true);
              }}
              className="border-theme-danger text-theme-danger rounded-[var(--radius-sm)] border px-2 py-1 text-xs transition-colors hover:opacity-80"
            >
              Delete
            </button>
          </>
        )}
      </td>
    </tr>
  );
}
