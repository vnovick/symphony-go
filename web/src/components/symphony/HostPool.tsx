import { useMemo } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { EMPTY_HOSTS as EMPTY_SSH_HOSTS, EMPTY_RUNNING } from '../../utils/constants';

interface HostEntry {
  id: string;
  name: string;
  description?: string;
  kind: 'local' | 'ssh';
  active: number;
  max: number;
  disabled?: boolean;
}

function loadBarColor(pct: number): string {
  if (pct >= 90) return '#ef4444';
  if (pct >= 75) return '#f59e0b';
  return 'var(--accent)';
}

function HostTile({ host }: { host: HostEntry }) {
  // SSH tiles show only the active count — max is a global pool cap, not per-host.
  // Showing "0 / 3" on every SSH tile would imply each host can hold 3 independently.
  const showMax = host.kind === 'local';
  const pct = showMax && host.max > 0 ? Math.round((host.active / host.max) * 100) : 0;
  const counter = showMax && host.max > 0 ? `${host.active} / ${host.max}` : String(host.active);

  const kindStyle =
    host.kind === 'local'
      ? { bg: 'rgba(34,197,94,0.12)', color: '#4ade80', label: 'Local' }
      : { bg: 'rgba(99,102,241,0.12)', color: '#818cf8', label: 'SSH' };

  return (
    <div
      className="rounded-xl border overflow-hidden bg-theme-bg-elevated border-theme-line"
    >
      <div
        className="flex items-center gap-2.5 px-3.5 py-3 border-b border-theme-line"
      >
        <span
          className={`inline-block w-1.5 h-1.5 rounded-full flex-shrink-0${host.disabled ? '' : ' animate-pulse'}`}
          style={{ background: host.disabled ? '#6b7280' : '#22c55e' }}
        />
        <span
          className="flex-1 min-w-0 truncate font-mono text-[12px] font-semibold"
          style={{ color: host.disabled ? 'var(--text-secondary)' : 'var(--text)' }}
          title={host.description ?? host.name}
        >
          {host.description ?? host.name}
        </span>
        {host.disabled ? (
          <span
            className="flex-shrink-0 px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-[0.04em]"
            style={{ background: 'rgba(239,68,68,0.12)', color: '#ef4444' }}
          >
            Disabled
          </span>
        ) : (
          <span
            className="flex-shrink-0 px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-[0.04em]"
            style={{ background: kindStyle.bg, color: kindStyle.color }}
          >
            {kindStyle.label}
          </span>
        )}
      </div>

      <div className="px-3.5 py-3">
        <div className="flex items-center justify-between mb-1.5">
          <span className="text-[11px] text-theme-text-secondary">
            Active agents
          </span>
          <span className="font-mono text-[12px] font-semibold text-theme-text">
            {counter}
          </span>
        </div>
        {showMax && (
          <div className="h-1 rounded-full overflow-hidden bg-theme-bg-soft">
            <div
              className="h-full rounded-full transition-all"
              style={{
                width: `${String(Math.min(pct, 100))}%`,
                background: host.disabled ? '#6b7280' : loadBarColor(pct),
              }}
            />
          </div>
        )}
        {host.kind === 'ssh' && (
          <p className="mt-1.5 text-[10px] font-mono truncate text-theme-muted">
            {host.name}
          </p>
        )}
        {host.disabled && (
          <p className="mt-1.5 text-[10px] leading-snug text-theme-muted">
            Agents route to SSH hosts
          </p>
        )}
      </div>
    </div>
  );
}

export function HostPool() {
  const running = useSymphonyStore((s) => s.snapshot?.running ?? EMPTY_RUNNING);
  const sshHosts = useSymphonyStore((s) => s.snapshot?.sshHosts ?? EMPTY_SSH_HOSTS);
  const maxConcurrentAgents = useSymphonyStore((s) => s.snapshot?.maxConcurrentAgents ?? 0);

  const hosts = useMemo<HostEntry[]>(() => {
    const localDisabled = sshHosts.length > 0;
    const local: HostEntry = {
      id: 'local',
      name: 'local',
      kind: 'local',
      active: running.filter((r) => !r.workerHost).length,
      max: maxConcurrentAgents,
      disabled: localDisabled,
    };
    const ssh: HostEntry[] = sshHosts.map((h) => ({
      id: `ssh:${h.host}`,
      name: h.host,
      description: h.description ?? undefined,
      kind: 'ssh' as const,
      active: running.filter((r) => r.workerHost === h.host).length,
      max: maxConcurrentAgents,
    }));
    return [local, ...ssh];
  }, [running, sshHosts, maxConcurrentAgents]);

  return (
    <div>
      <span
        className="text-[11px] font-semibold uppercase tracking-[0.06em] mb-2 block"
      >
        Host Pool
      </span>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {hosts.map((host) => (
          <HostTile key={host.id} host={host} />
        ))}
      </div>
    </div>
  );
}
