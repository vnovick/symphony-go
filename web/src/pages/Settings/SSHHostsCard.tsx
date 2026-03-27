import { useState } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useSettingsActions } from '../../hooks/useSettingsActions';
import { AddSSHHostModal } from './AddSSHHostModal';
import type { SSHHostInfo } from '../../types/schemas';

const STRATEGIES: { id: string; label: string; desc: string; available: boolean }[] = [
  { id: 'round-robin',  label: 'Round Robin',  desc: 'Cycle hosts in order',                        available: true  },
  { id: 'least-loaded', label: 'Least Loaded', desc: 'Route to host with fewest active agents',     available: true  },
];

const EMPTY_HOSTS: SSHHostInfo[] = [];

export function SSHHostsCard() {
  const hosts = useSymphonyStore((s) => s.snapshot?.sshHosts ?? EMPTY_HOSTS);
  const strategy = useSymphonyStore((s) => s.snapshot?.dispatchStrategy ?? 'round-robin');
  const { addSSHHost, removeSSHHost, setDispatchStrategy } = useSettingsActions();
  const [addOpen, setAddOpen] = useState(false);
  const [removingHost, setRemovingHost] = useState<string | null>(null);

  const handleRemove = async (host: string) => {
    setRemovingHost(host);
    await removeSSHHost(host);
    setRemovingHost(null);
  };

  return (
    <>
      <div
        className="rounded-xl border overflow-hidden bg-theme-bg-elevated border-theme-line"
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-[18px] py-3.5 border-b border-theme-line"
        >
          <div>
            <span className="text-[15px] font-semibold text-theme-text">
              SSH Hosts
            </span>
            <span
              className="ml-2 text-[11px]"
            >
              {hosts.length === 0 ? 'running locally' : `${hosts.length} host${hosts.length === 1 ? '' : 's'}`}
            </span>
          </div>
          <button
            onClick={() => { setAddOpen(true); }}
            style={{
              padding: '5px 12px',
              borderRadius: 6,
              fontSize: 12,
              fontWeight: 600,
              cursor: 'pointer',
              background: 'var(--accent)',
              color: '#fff',
              border: 'none',
            }}
          >
            + Add host
          </button>
        </div>

        {/* Host list */}
        {hosts.length === 0 ? (
          <div className="px-[18px] py-5 text-[13px] text-theme-muted">
            No SSH hosts configured — agents run locally on this machine.
          </div>
        ) : (
          <ul className="divide-y border-theme-line">
            {hosts.map((h) => (
              <li
                key={h.host}
                className="flex items-center justify-between px-[18px] py-3"
              >
                <div className="min-w-0">
                  <span className="font-mono text-[13px] text-theme-text">
                    {h.host}
                  </span>
                  {h.description && (
                    <span className="ml-2 text-[12px] text-theme-muted">
                      {h.description}
                    </span>
                  )}
                </div>
                <button
                  onClick={() => { void handleRemove(h.host); }}
                  disabled={removingHost === h.host}
                  className="ml-4 flex-shrink-0 text-[12px] transition-opacity"
                  style={{
                    color: 'var(--danger)',
                    background: 'transparent',
                    border: 'none',
                    cursor: removingHost === h.host ? 'wait' : 'pointer',
                    opacity: removingHost === h.host ? 0.5 : 1,
                  }}
                >
                  {removingHost === h.host ? 'Removing…' : 'Remove'}
                </button>
              </li>
            ))}
          </ul>
        )}

        {/* Dispatch strategy — only shown when there are hosts */}
        {hosts.length > 0 && (
          <div className="border-t px-[18px] py-4 border-theme-line">
            <p className="text-[12px] font-medium mb-2.5 text-theme-text-secondary">
              Dispatch strategy
            </p>
            <div className="flex gap-2">
              {STRATEGIES.map((s) => (
                <button
                  key={s.id}
                  onClick={() => { void setDispatchStrategy(s.id); }}
                  className="flex-1 text-left px-3 py-2.5 rounded-lg border-2 transition-all"
                  style={{
                    borderColor: strategy === s.id ? 'var(--accent)' : 'var(--line)',
                    background: strategy === s.id ? 'rgba(99,102,241,0.06)' : 'transparent',
                  }}
                >
                  <div className="text-[12px] font-semibold text-theme-text">
                    {s.label}
                  </div>
                  <div className="text-[11px] mt-0.5 leading-[1.4] text-theme-text-secondary">
                    {s.desc}
                  </div>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>

      <AddSSHHostModal
        isOpen={addOpen}
        onClose={() => { setAddOpen(false); }}
        onAdd={addSSHHost}
      />
    </>
  );
}
