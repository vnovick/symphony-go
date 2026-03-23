import { useSymphonyStore } from '../../store/symphonyStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';
import PageMeta from '../../components/common/PageMeta';
import type { ProfileDef } from '../../types/symphony';
import { ProfilesCard } from './ProfilesCard';
import { TrackerStatesCard } from './TrackerStatesCard';
import { WorkspaceCard } from './WorkspaceCard';
import { WorkflowReferenceCard } from './WorkflowReferenceCard';

const EMPTY_PROFILE_DEFS: Record<string, ProfileDef> = {};

export default function Settings() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const profileDefs = useSymphonyStore((s) => s.snapshot?.profileDefs ?? EMPTY_PROFILE_DEFS);
  const { upsertProfile, deleteProfile, updateTrackerStates, setAutoClearWorkspace } =
    useSettingsActions();

  const activeStatesKey = snapshot?.activeStates?.join(',') ?? '';
  const terminalStatesKey = snapshot?.terminalStates?.join(',') ?? '';
  const completionStateValue = snapshot?.completionState ?? '';

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

        <ProfilesCard profileDefs={profileDefs} onUpsert={upsertProfile} onDelete={deleteProfile} />

        <TrackerStatesCard
          key={`${activeStatesKey}|${terminalStatesKey}|${completionStateValue}`}
          initialActiveStates={snapshot?.activeStates ?? []}
          initialTerminalStates={snapshot?.terminalStates ?? []}
          initialCompletionState={snapshot?.completionState ?? ''}
          onSave={updateTrackerStates}
        />

        <WorkspaceCard
          autoClearWorkspace={snapshot?.autoClearWorkspace ?? false}
          onToggle={setAutoClearWorkspace}
        />

        <WorkflowReferenceCard />
      </div>
    </>
  );
}
