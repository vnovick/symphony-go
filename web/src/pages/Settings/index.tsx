import { useShallow } from 'zustand/react/shallow';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';
import PageMeta from '../../components/common/PageMeta';
import type { ProfileDef } from '../../types/schemas';
import { ProfilesCard } from './ProfilesCard';
import { TrackerStatesCard } from './TrackerStatesCard';
import { WorkspaceCard } from './WorkspaceCard';
import { ProjectFilterCard } from './ProjectFilterCard';
import { SSHHostsCard } from './SSHHostsCard';
import { CapacityCard } from './CapacityCard';
import { ConfirmButton } from '../../components/ui/button/ConfirmButton';
import { useClearAllLogs, useClearAllWorkspaces } from '../../queries/issues';
import { useQueryClient } from '@tanstack/react-query';

const EMPTY_PROFILE_DEFS: Record<string, ProfileDef> = {};
const EMPTY_STATES: string[] = [];

export default function Settings() {
  const { activeStates, terminalStates, completionState, autoClearWorkspace } = useSymphonyStore(
    useShallow((s) => ({
      activeStates: s.snapshot?.activeStates ?? EMPTY_STATES,
      terminalStates: s.snapshot?.terminalStates ?? EMPTY_STATES,
      completionState: s.snapshot?.completionState ?? '',
      autoClearWorkspace: s.snapshot?.autoClearWorkspace ?? false,
    })),
  );
  const profileDefs = useSymphonyStore((s) => s.snapshot?.profileDefs ?? EMPTY_PROFILE_DEFS);
  const {
    upsertProfile,
    deleteProfile,
    updateTrackerStates,
    setAutoClearWorkspace,
    setProjectFilter,
  } = useSettingsActions();
  const queryClient = useQueryClient();
  const clearAllLogs = useClearAllLogs();
  const clearAllWorkspaces = useClearAllWorkspaces();
  const trackerKind = useSymphonyStore((s) => s.snapshot?.trackerKind);
  const activeProjectFilter = useSymphonyStore((s) => s.snapshot?.activeProjectFilter);


  return (
    <>
      <PageMeta
        title="Symphony | Settings"
        description="Symphony settings — profiles, tracker states, and workspace"
      />
      <div className="max-w-3xl space-y-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-theme-text">
            Settings
          </h1>
          <p className="mt-1 text-sm text-theme-muted">
            Configure agent profiles, tracker states, and workspace behaviour. All settings are also
            hot-reloaded from{' '}
            <code
              className="rounded px-1.5 py-0.5 font-mono text-xs bg-theme-bg-soft text-theme-accent"
            >
              WORKFLOW.md
            </code>
            .
          </p>
        </div>

        {/* ── Profiles ──────────────────────────────────────────────────────── */}
        <section aria-labelledby="section-profiles">
          <h2
            id="section-profiles"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            Profiles
          </h2>
          <ProfilesCard
            profileDefs={profileDefs}
            onUpsert={upsertProfile}
            onDelete={deleteProfile}
          />
        </section>

        {/* ── Tracker States ────────────────────────────────────────────────── */}
        <section aria-labelledby="section-tracker">
          <h2
            id="section-tracker"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            Tracker States
          </h2>
          <div className="space-y-4">
            <TrackerStatesCard
              initialActiveStates={activeStates}
              initialTerminalStates={terminalStates}
              initialCompletionState={completionState}
              onSave={updateTrackerStates}
            />
            {trackerKind === 'linear' && (
              <ProjectFilterCard
                activeFilter={activeProjectFilter}
                onSetFilter={setProjectFilter}
              />
            )}
          </div>
        </section>

        {/* ── Workspace ─────────────────────────────────────────────────────── */}
        <section aria-labelledby="section-workspace">
          <h2
            id="section-workspace"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            Workspace
          </h2>
          <WorkspaceCard
            autoClearWorkspace={autoClearWorkspace}
            onToggle={setAutoClearWorkspace}
          />
        </section>

        {/* ── Agents ────────────────────────────────────────────────────── */}
        <section aria-labelledby="section-agents">
          <h2
            id="section-agents"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            Agents
          </h2>
          <CapacityCard />
        </section>

        {/* ── SSH Hosts ─────────────────────────────────────────────────── */}
        <section aria-labelledby="section-ssh-hosts">
          <h2
            id="section-ssh-hosts"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            SSH Hosts
          </h2>
          <SSHHostsCard />
        </section>

        {/* ── Logs ──────────────────────────────────────────────────────────── */}
        <section aria-labelledby="section-logs">
          <h2
            id="section-logs"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            Logs
          </h2>
          <div
            className="space-y-3 rounded-lg border p-4 border-theme-line bg-theme-panel"
          >
            {/* Clear all logs */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-theme-text">
                  Clear all logs
                </p>
                <p className="mt-0.5 text-xs text-theme-muted">
                  Deletes in-memory and on-disk log buffers for all issues.
                </p>
              </div>
              <ConfirmButton
                label="Clear all logs"
                confirmLabel="Yes, clear"
                pendingLabel="Clearing…"
                isPending={clearAllLogs.isPending}
                onConfirm={() => { clearAllLogs.mutate(undefined); }}
              />
            </div>

            <div className="border-t border-theme-line" />

            {/* Reset all workspaces */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-theme-text">
                  Reset all workspaces
                </p>
                <p className="mt-0.5 text-xs text-theme-muted">
                  Deletes all cloned workspace directories under workspace.root. Does not affect logs
                  or tracker state.
                </p>
              </div>
              <ConfirmButton
                label="Reset workspaces"
                confirmLabel="Yes, reset"
                pendingLabel="Resetting…"
                isPending={clearAllWorkspaces.isPending}
                onConfirm={() => {
                  clearAllWorkspaces.mutate(undefined, {
                    onSuccess: () => {
                      void useSymphonyStore.getState().refreshSnapshot();
                      void queryClient.invalidateQueries({ queryKey: ['logs'] });
                      void queryClient.invalidateQueries({ queryKey: ['sublogs'] });
                    },
                  });
                }}
              />
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
