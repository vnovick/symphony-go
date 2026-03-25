import { useState } from 'react';
import { Modal } from '../../components/ui/modal';

interface AddSSHHostModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (host: string, description: string) => Promise<boolean>;
}

export function AddSSHHostModal({ isOpen, onClose, onAdd }: AddSSHHostModalProps) {
  const [host, setHost] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [hostType, setHostType] = useState<'ssh' | 'docker'>('ssh');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!host.trim()) return;
    setSaving(true);
    const ok = await onAdd(host.trim(), description.trim());
    setSaving(false);
    if (ok) {
      setHost('');
      setDescription('');
      onClose();
    }
  };

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '8px 10px',
    borderRadius: 6,
    border: '1px solid var(--line)',
    background: 'var(--bg-soft)',
    color: 'var(--text)',
    fontSize: 13,
    outline: 'none',
  };

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: 12,
    fontWeight: 500,
    marginBottom: 4,
    color: 'var(--text-secondary)',
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} showCloseButton className="max-w-md p-6">
      <h2 className="text-base font-semibold mb-4" style={{ color: 'var(--text)' }}>
        Add Worker Host
      </h2>

      {/* Host type selector */}
      <div className="flex gap-2 mb-5">
        <button
          type="button"
          onClick={() => { setHostType('ssh'); }}
          className="flex-1 py-2.5 px-3 rounded-lg border-2 text-left transition-all"
          style={{
            borderColor: hostType === 'ssh' ? 'var(--accent)' : 'var(--line)',
            background: hostType === 'ssh' ? 'rgba(99,102,241,0.06)' : 'transparent',
          }}
        >
          <div className="text-[13px] font-semibold" style={{ color: 'var(--text)' }}>SSH</div>
          <div className="text-[11px] mt-0.5" style={{ color: 'var(--text-secondary)' }}>
            Remote host via SSH
          </div>
        </button>
        <button
          type="button"
          disabled
          className="flex-1 py-2.5 px-3 rounded-lg border-2 text-left opacity-50 cursor-not-allowed"
          style={{ borderColor: 'var(--line)', background: 'transparent' }}
          title="Coming in a future release"
        >
          <div className="flex items-center gap-1.5">
            <span className="text-[13px] font-semibold" style={{ color: 'var(--text)' }}>Docker</span>
            <span
              className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide"
              style={{ background: 'rgba(99,102,241,0.12)', color: '#818cf8' }}
            >
              Soon
            </span>
          </div>
          <div className="text-[11px] mt-0.5" style={{ color: 'var(--text-secondary)' }}>
            Ephemeral containers
          </div>
        </button>
      </div>

      <form onSubmit={(e) => { void handleSubmit(e); }} className="space-y-4">
        <div>
          <label style={labelStyle}>
            Host address <span style={{ color: 'var(--danger)' }}>*</span>
          </label>
          <input
            style={inputStyle}
            type="text"
            value={host}
            onChange={(e) => { setHost(e.target.value); }}
            placeholder="build-server.example.com or 192.168.1.10:22"
            autoFocus
            required
          />
          <p className="mt-1 text-[11px]" style={{ color: 'var(--muted)' }}>
            Use <code style={{ background: 'var(--bg-soft)', padding: '0 3px', borderRadius: 3 }}>host</code> or{' '}
            <code style={{ background: 'var(--bg-soft)', padding: '0 3px', borderRadius: 3 }}>host:port</code>.
            Defaults to port 22.
          </p>
        </div>

        <div>
          <label style={labelStyle}>Description (optional)</label>
          <input
            style={inputStyle}
            type="text"
            value={description}
            onChange={(e) => { setDescription(e.target.value); }}
            placeholder="e.g. Build server — 32 cores, 64 GB RAM"
          />
        </div>

        {/* Host key warning */}
        <div
          className="rounded-lg px-3.5 py-3 text-[12px] leading-relaxed space-y-1.5"
          style={{ background: 'rgba(234,179,8,0.08)', border: '1px solid rgba(234,179,8,0.25)', color: '#ca8a04' }}
        >
          <div className="font-semibold flex items-center gap-1.5">
            <span>⚠</span> SSH host key required
          </div>
          <p style={{ color: '#a16207' }}>
            The host's key must be in{' '}
            <code style={{ background: 'rgba(234,179,8,0.12)', padding: '0 3px', borderRadius: 3 }}>
              ~/.ssh/known_hosts
            </code>{' '}
            on this machine before Symphony can connect. Run once to pre-accept it:
          </p>
          <pre
            className="rounded px-2.5 py-1.5 text-[11px] font-mono select-all"
            style={{ background: 'rgba(0,0,0,0.15)', color: '#fbbf24' }}
          >
            {`ssh-keyscan -H ${host.trim() || '<host>'} >> ~/.ssh/known_hosts`}
          </pre>
        </div>

        <div className="flex justify-end gap-2 pt-1">
          <button
            type="button"
            onClick={onClose}
            style={{
              padding: '7px 16px',
              borderRadius: 6,
              fontSize: 13,
              cursor: 'pointer',
              background: 'transparent',
              color: 'var(--text-secondary)',
              border: '1px solid var(--line)',
            }}
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={saving || !host.trim()}
            style={{
              padding: '7px 16px',
              borderRadius: 6,
              fontSize: 13,
              fontWeight: 600,
              cursor: saving || !host.trim() ? 'not-allowed' : 'pointer',
              background: 'var(--accent)',
              color: '#fff',
              border: 'none',
              opacity: saving || !host.trim() ? 0.6 : 1,
            }}
          >
            {saving ? 'Adding…' : 'Add host'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
