import { useState, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import type { ProfileDef } from '../../types/schemas';
import {
  applyBackendSelection,
  applyModelSelection,
  buildCanonicalCommand,
  commandToBackend,
  commandToModel,
  inferBackendFromCommand,
  normalizeCommandForSave,
} from './profileCommands';
import { ProfileEditorFields } from './profiles/ProfileEditorFields';
import { ProfileRow } from './profiles/ProfileRow';
import { SuggestedProfileCard, TemplatePreviewModal } from './profiles/SuggestedProfileCard';
import { SUGGESTED_PROFILES, type SuggestedProfile } from './profiles/suggestedProfiles';

// ─── Zod schema for add form ─────────────────────────────────────────────────

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

type AddProfileValues = z.infer<typeof addProfileSchema>;

// ─── ProfilesCard ─────────────────────────────────────────────────────────────

interface ProfilesCardProps {
  profileDefs: Record<string, ProfileDef>;
  onUpsert: (name: string, command: string, backend?: string, prompt?: string) => Promise<boolean>;
  onDelete: (name: string) => Promise<boolean>;
  availableModels?: Record<string, { id: string; label: string }[]>;
}

export function ProfilesCard({
  profileDefs,
  onUpsert,
  onDelete,
  availableModels,
}: ProfilesCardProps) {
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
      <div className="border-theme-line bg-theme-bg-elevated overflow-hidden rounded-[var(--radius-md)] border">
        <div className="border-theme-line bg-theme-panel-strong flex items-center justify-between border-b px-5 py-4">
          <div>
            <h2 className="text-theme-text text-sm font-semibold">Agent Profiles</h2>
            <p className="text-theme-text-secondary mt-0.5 text-xs">
              Select per-issue from the issue detail modal. Backend and model controls stay
              backend-aware, and custom wrapper commands are preserved instead of flattened.
            </p>
          </div>
          {!adding && (
            <button
              onClick={openAddForm}
              className="bg-theme-accent flex items-center gap-1.5 rounded-[var(--radius-sm)] px-3 py-1.5 text-xs font-medium text-white transition-colors hover:opacity-90"
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
            <thead className="bg-theme-bg-soft">
              <tr>
                <th className="w-40 px-4 py-3 text-left text-xs font-medium tracking-wider uppercase">
                  Name
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium tracking-wider uppercase">
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
                  availableModels={availableModels}
                />
              ))}

              {adding && (
                <tr className="border-theme-line bg-theme-bg-soft border-b">
                  <td className="px-4 py-3 align-top">
                    <input
                      className="w-full rounded border px-3 py-1.5 font-mono text-sm focus:ring-1 focus:outline-none"
                      style={{
                        borderColor: 'var(--line)',
                        background: 'var(--panel-strong)',
                        color: 'var(--text)',
                      }}
                      placeholder="profile-name"
                      {...addForm.register('name')}
                      onKeyDown={(e) => {
                        if (e.key === 'Escape') handleAddCancel();
                      }}
                      autoFocus
                    />
                    {addForm.formState.errors.name && (
                      <p role="alert" className="text-theme-danger mt-1 text-xs">
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
                        addForm.setValue('backend', value, { shouldValidate: true });
                        addForm.setValue('model', next.model);
                        addForm.setValue('command', next.command, { shouldValidate: true });
                      }}
                      onModelChange={(value) => {
                        addForm.setValue('model', value);
                        addForm.setValue(
                          'command',
                          applyModelSelection(addCommand, addBackend, value),
                          { shouldValidate: true },
                        );
                      }}
                      onCommandChange={(value) => {
                        addForm.setValue('command', value, { shouldValidate: true });
                        addForm.setValue('model', commandToModel(value));
                        const inferred = inferBackendFromCommand(value);
                        if (inferred) addForm.setValue('backend', inferred);
                      }}
                      onPromptChange={(value) => {
                        addForm.setValue('prompt', value);
                      }}
                      dynamicModels={availableModels}
                    />
                    {addForm.formState.errors.command && (
                      <p role="alert" className="text-theme-danger text-xs">
                        {addForm.formState.errors.command.message}
                      </p>
                    )}
                    {addForm.formState.errors.root && (
                      <p role="alert" className="text-theme-danger text-xs">
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
                      className="bg-theme-accent mr-2 rounded-[var(--radius-sm)] px-3 py-1 text-sm text-white transition-colors disabled:opacity-50"
                    >
                      {addForm.formState.isSubmitting ? 'Saving…' : 'Save'}
                    </button>
                    <button
                      onClick={handleAddCancel}
                      className="border-theme-line text-theme-text-secondary rounded-[var(--radius-sm)] border px-3 py-1 text-sm transition-colors hover:opacity-80"
                    >
                      Cancel
                    </button>
                  </td>
                </tr>
              )}

              {profileEntries.length === 0 && !adding && (
                <tr>
                  <td colSpan={3} className="px-4 py-10 text-center text-sm">
                    No profiles configured yet.{' '}
                    <button onClick={openAddForm} className="text-theme-accent hover:underline">
                      Add one
                    </button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {suggestedToShow.length > 0 && (
          <div className="border-theme-line border-t px-6 py-4">
            <p className="text-theme-muted mb-3 text-[11px] font-medium tracking-wider uppercase">
              Quick-add templates
            </p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
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

      {deleteError && <p className="text-theme-danger text-sm">{deleteError}</p>}

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
