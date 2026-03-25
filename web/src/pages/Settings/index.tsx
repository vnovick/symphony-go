import { useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';
import PageMeta from '../../components/common/PageMeta';
import type { ProfileDef } from '../../types/symphony';
import { ProfilesCard } from './ProfilesCard';
import { TrackerStatesCard } from './TrackerStatesCard';
import { WorkspaceCard } from './WorkspaceCard';
import { ProjectFilterCard } from './ProjectFilterCard';
import { SSHHostsCard } from './SSHHostsCard';
import { CapacityCard } from './CapacityCard';
import { useClearAllLogs, useClearAllWorkspaces } from '../../queries/issues';
import { useQueryClient } from '@tanstack/react-query';

const EMPTY_PROFILE_DEFS: Record<string, ProfileDef> = {};

export default function Settings() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
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
  const [confirmClearLogs, setConfirmClearLogs] = useState(false);
  const [confirmResetWorkspaces, setConfirmResetWorkspaces] = useState(false);
  const trackerKind = useSymphonyStore((s) => s.snapshot?.trackerKind);
  const activeProjectFilter = useSymphonyStore((s) => s.snapshot?.activeProjectFilter);

  const activeStatesKey = JSON.stringify(snapshot?.activeStates ?? []);
  const terminalStatesKey = JSON.stringify(snapshot?.terminalStates ?? []);
  const completionStateValue = snapshot?.completionState ?? '';

  return (
    <>
      <PageMeta
        title="Symphony | Settings"
        description="Symphony settings — profiles, tracker states, and workspace"
      />
      <div className="max-w-3xl space-y-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight" style={{ color: 'var(--text)' }}>
            Settings
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--muted)' }}>
            Configure agent profiles, tracker states, and workspace behaviour. All settings are also
            hot-reloaded from{' '}
            <code
              className="rounded px-1.5 py-0.5 font-mono text-xs"
              style={{ background: 'var(--bg-soft)', color: 'var(--accent)' }}
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
            style={{ color: 'var(--muted)' }}
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
            style={{ color: 'var(--muted)' }}
          >
            Tracker States
          </h2>
          <div className="space-y-4">
            <TrackerStatesCard
              key={`${activeStatesKey}|${terminalStatesKey}|${completionStateValue}`}
              initialActiveStates={snapshot?.activeStates ?? []}
              initialTerminalStates={snapshot?.terminalStates ?? []}
              initialCompletionState={snapshot?.completionState ?? ''}
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
            style={{ color: 'var(--muted)' }}
          >
            Workspace
          </h2>
          <WorkspaceCard
            autoClearWorkspace={snapshot?.autoClearWorkspace ?? false}
            onToggle={setAutoClearWorkspace}
          />
        </section>

        {/* ── Agents ────────────────────────────────────────────────────── */}
        <section aria-labelledby="section-agents">
          <h2
            id="section-agents"
            className="mb-3 text-xs font-semibold tracking-widest uppercase"
            style={{ color: 'var(--muted)' }}
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
            style={{ color: 'var(--muted)' }}
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
            style={{ color: 'var(--muted)' }}
          >
            Logs
          </h2>
          <div
            className="space-y-3 rounded-lg border p-4"
            style={{ borderColor: 'var(--line)', background: 'var(--panel)' }}
          >
            {/* Clear all logs */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium" style={{ color: 'var(--text)' }}>
                  Clear all logs
                </p>
                <p className="mt-0.5 text-xs" style={{ color: 'var(--muted)' }}>
                  Deletes in-memory and on-disk log buffers for all issues.
                </p>
              </div>
              {confirmClearLogs ? (
                <div className="flex items-center gap-2">
                  <span className="text-xs" style={{ color: 'var(--muted)' }}>
                    Are you sure?
                  </span>
                  <button
                    onClick={() => {
                      clearAllLogs.mutate(undefined, {
                        onSuccess: () => {
                          setConfirmClearLogs(false);
                        },
                      });
                    }}
                    disabled={clearAllLogs.isPending}
                    style={{
                      padding: '4px 10px',
                      borderRadius: 4,
                      fontSize: 12,
                      fontWeight: 600,
                      cursor: clearAllLogs.isPending ? 'wait' : 'pointer',
                      background: 'var(--danger)',
                      color: '#fff',
                      border: 'none',
                    }}
                  >
                    {clearAllLogs.isPending ? 'Clearing…' : 'Yes, clear'}
                  </button>
                  <button
                    onClick={() => {
                      setConfirmClearLogs(false);
                    }}
                    style={{
                      padding: '4px 10px',
                      borderRadius: 4,
                      fontSize: 12,
                      cursor: 'pointer',
                      background: 'transparent',
                      color: 'var(--text-secondary)',
                      border: '1px solid var(--line)',
                    }}
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => {
                    setConfirmClearLogs(true);
                  }}
                  style={{
                    padding: '6px 14px',
                    borderRadius: 4,
                    fontSize: 12,
                    fontWeight: 500,
                    cursor: 'pointer',
                    background: 'transparent',
                    color: 'var(--danger)',
                    border: '1px solid var(--danger)',
                  }}
                >
                  Clear all logs
                </button>
              )}
            </div>

            <div style={{ borderTop: '1px solid var(--line)' }} />

            {/* Reset all workspaces */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium" style={{ color: 'var(--text)' }}>
                  Reset all workspaces
                </p>
                <p className="mt-0.5 text-xs" style={{ color: 'var(--muted)' }}>
                  Deletes all cloned workspace directories under workspace.root. Does not affect logs
                  or tracker state.
                </p>
              </div>
              {confirmResetWorkspaces ? (
                <div className="flex items-center gap-2">
                  <span className="text-xs" style={{ color: 'var(--muted)' }}>
                    Are you sure?
                  </span>
                  <button
                    onClick={() => {
                      clearAllWorkspaces.mutate(undefined, {
                        onSuccess: () => {
                          setConfirmResetWorkspaces(false);
                          void useSymphonyStore.getState().refreshSnapshot();
                          void queryClient.invalidateQueries({ queryKey: ['logs'] });
                          void queryClient.invalidateQueries({ queryKey: ['sublogs'] });
                        },
                      });
                    }}
                    disabled={clearAllWorkspaces.isPending}
                    style={{
                      padding: '4px 10px',
                      borderRadius: 4,
                      fontSize: 12,
                      fontWeight: 600,
                      cursor: clearAllWorkspaces.isPending ? 'wait' : 'pointer',
                      background: 'var(--danger)',
                      color: '#fff',
                      border: 'none',
                    }}
                  >
                    {clearAllWorkspaces.isPending ? 'Resetting…' : 'Yes, reset'}
                  </button>
                  <button
                    onClick={() => {
                      setConfirmResetWorkspaces(false);
                    }}
                    style={{
                      padding: '4px 10px',
                      borderRadius: 4,
                      fontSize: 12,
                      cursor: 'pointer',
                      background: 'transparent',
                      color: 'var(--text-secondary)',
                      border: '1px solid var(--line)',
                    }}
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => {
                    setConfirmResetWorkspaces(true);
                  }}
                  style={{
                    padding: '6px 14px',
                    borderRadius: 4,
                    fontSize: 12,
                    fontWeight: 500,
                    cursor: 'pointer',
                    background: 'transparent',
                    color: 'var(--danger)',
                    border: '1px solid var(--danger)',
                  }}
                >
                  Reset workspaces
                </button>
              )}
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
