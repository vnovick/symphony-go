import { useShallow } from 'zustand/react/shallow';
import { useItervoxStore } from '../../store/itervoxStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';
import PageMeta from '../../components/common/PageMeta';
import { ProfilesCard } from './ProfilesCard';
import { TrackerStatesCard } from './TrackerStatesCard';
import { WorkspaceCard } from './WorkspaceCard';
import { ProjectFilterCard } from './ProjectFilterCard';
import { SSHHostsCard } from './SSHHostsCard';
import { ReviewerCard } from './ReviewerCard';
import { CapacityCard } from './CapacityCard';
import { ConfirmButton } from '../../components/ui/button/ConfirmButton';
import { useClearAllLogs, useClearAllWorkspaces } from '../../queries/issues';
import { EMPTY_PROFILE_DEFS, EMPTY_PROFILES, EMPTY_STATES } from '../../utils/constants';

export default function Settings() {
  const { activeStates, terminalStates, completionState, autoClearWorkspace } = useItervoxStore(
    useShallow((s) => ({
      activeStates: s.snapshot?.activeStates ?? EMPTY_STATES,
      terminalStates: s.snapshot?.terminalStates ?? EMPTY_STATES,
      completionState: s.snapshot?.completionState ?? '',
      autoClearWorkspace: s.snapshot?.autoClearWorkspace ?? false,
    })),
  );
  const profileDefs = useItervoxStore((s) => s.snapshot?.profileDefs ?? EMPTY_PROFILE_DEFS);
  const availableModels = useItervoxStore((s) => s.snapshot?.availableModels);
  const availableProfiles = useItervoxStore((s) => s.snapshot?.availableProfiles ?? EMPTY_PROFILES);
  const reviewerProfile = useItervoxStore((s) => s.snapshot?.reviewerProfile ?? '');
  const autoReview = useItervoxStore((s) => s.snapshot?.autoReview ?? false);
  const {
    upsertProfile,
    deleteProfile,
    updateTrackerStates,
    setAutoClearWorkspace,
    setProjectFilter,
    setReviewerConfig,
  } = useSettingsActions();
  const clearAllLogs = useClearAllLogs();
  const clearAllWorkspaces = useClearAllWorkspaces();
  const trackerKind = useItervoxStore((s) => s.snapshot?.trackerKind);
  const activeProjectFilter = useItervoxStore((s) => s.snapshot?.activeProjectFilter);
  const autoReviewEnabled = autoReview && reviewerProfile !== '';

  return (
    <>
      <PageMeta
        title="Itervox | Settings"
        description="Itervox settings — profiles, tracker states, and workspace"
      />
      <div className="max-w-3xl space-y-8">
        <div>
          <h1 className="text-theme-text text-2xl font-bold tracking-tight">Settings</h1>
          <p className="text-theme-muted mt-1 text-sm">
            Configure agent profiles, tracker states, and workspace behaviour. All settings are also
            hot-reloaded from{' '}
            <code className="bg-theme-bg-soft text-theme-accent rounded px-1.5 py-0.5 font-mono text-xs">
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
            availableModels={availableModels}
          />
        </section>

        {/* ── Code Review Agent ────────────────────────────────────────────── */}
        <section aria-labelledby="section-reviewer">
          <h2
            id="section-reviewer"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
          >
            Code Review Agent
          </h2>
          <ReviewerCard
            reviewerProfile={reviewerProfile}
            autoReview={autoReview}
            autoClearWorkspace={autoClearWorkspace}
            availableProfiles={availableProfiles}
            onSave={setReviewerConfig}
          />
        </section>

        {/* ── Tracker States ────────────────────────────────────────────────── */}
        <section aria-labelledby="section-tracker">
          <h2 id="section-tracker" className="mb-3 text-xs font-semibold tracking-widest uppercase">
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
            autoReviewEnabled={autoReviewEnabled}
            onToggle={setAutoClearWorkspace}
          />
        </section>

        {/* ── Agents ────────────────────────────────────────────────────── */}
        <section aria-labelledby="section-agents">
          <h2 id="section-agents" className="mb-3 text-xs font-semibold tracking-widest uppercase">
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
          <h2 id="section-logs" className="mb-3 text-xs font-semibold tracking-widest uppercase">
            Logs
          </h2>
          <div className="border-theme-line bg-theme-panel space-y-3 rounded-lg border p-4">
            {/* Clear all logs */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-theme-text text-sm font-medium">Clear all logs</p>
                <p className="text-theme-muted mt-0.5 text-xs">
                  Deletes in-memory and on-disk log buffers for all issues.
                </p>
              </div>
              <ConfirmButton
                label="Clear all logs"
                confirmLabel="Yes, clear"
                pendingLabel="Clearing…"
                isPending={clearAllLogs.isPending}
                onConfirm={() => {
                  clearAllLogs.mutate(undefined);
                }}
              />
            </div>

            <div className="border-theme-line border-t" />

            {/* Reset all workspaces */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-theme-text text-sm font-medium">Reset all workspaces</p>
                <p className="text-theme-muted mt-0.5 text-xs">
                  Deletes all cloned workspace directories under workspace.root. Does not affect
                  logs or tracker state.
                </p>
              </div>
              <ConfirmButton
                label="Reset workspaces"
                confirmLabel="Yes, reset"
                pendingLabel="Resetting…"
                isPending={clearAllWorkspaces.isPending}
                onConfirm={() => {
                  clearAllWorkspaces.mutate(undefined);
                }}
              />
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
