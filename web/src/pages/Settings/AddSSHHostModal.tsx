import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Modal } from '../../components/ui/modal';

// ─── Zod schema — mirrors Go backend validation in config/validate.go ────────

export const sshHostSchema = z.object({
  host: z
    .string()
    .min(1, 'Host address is required.')
    .regex(/^\S+$/, 'Must not contain spaces.')
    .refine((h) => !h.startsWith('-'), 'Must not start with a dash.'),
  description: z.string(),
});

export type SSHHostFormValues = z.infer<typeof sshHostSchema>;

// ─── Component ───────────────────────────────────────────────────────────────

interface AddSSHHostModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (host: string, description: string) => Promise<boolean>;
}

const inputCls =
  'w-full rounded-md border border-theme-line bg-theme-bg-soft text-theme-text text-[13px] px-2.5 py-2 outline-none';
const labelCls = 'block text-xs font-medium mb-1 text-theme-text-secondary';

export function AddSSHHostModal({ isOpen, onClose, onAdd }: AddSSHHostModalProps) {
  const {
    register,
    handleSubmit,
    watch,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<SSHHostFormValues>({
    resolver: zodResolver(sshHostSchema),
    defaultValues: { host: '', description: '' },
  });

  const hostValue = watch('host');

  const onSubmit = handleSubmit(async (values) => {
    const ok = await onAdd(values.host.trim(), values.description.trim());
    if (ok) {
      reset();
      onClose();
    }
  });

  return (
    <Modal isOpen={isOpen} onClose={onClose} showCloseButton className="max-w-md p-6">
      <h2 className="text-base font-semibold mb-4 text-theme-text">
        Add Worker Host
      </h2>

      {/* Host type selector */}
      <div className="flex gap-2 mb-5">
        <div
          className="flex-1 py-2.5 px-3 rounded-lg border-2 text-left"
          style={{
            borderColor: 'var(--accent)',
            background: 'rgba(99,102,241,0.06)',
          }}
        >
          <div className="text-[13px] font-semibold text-theme-text">SSH</div>
          <div className="text-[11px] mt-0.5 text-theme-text-secondary">
            Remote host via SSH
          </div>
        </div>
        <button
          type="button"
          disabled
          className="flex-1 py-2.5 px-3 rounded-lg border-2 text-left opacity-50 cursor-not-allowed"
          style={{ borderColor: 'var(--line)', background: 'transparent' }}
          title="Coming in a future release"
        >
          <div className="flex items-center gap-1.5">
            <span className="text-[13px] font-semibold text-theme-text">Docker</span>
            <span
              className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide"
              style={{ background: 'rgba(99,102,241,0.12)', color: '#818cf8' }}
            >
              Soon
            </span>
          </div>
          <div className="text-[11px] mt-0.5 text-theme-text-secondary">
            Ephemeral containers
          </div>
        </button>
      </div>

      <form onSubmit={(e) => { void onSubmit(e); }} className="space-y-4">
        <div>
          <label className={labelCls}>
            Host address <span className="text-theme-danger">*</span>
          </label>
          <input
            className={inputCls}
            type="text"
            placeholder="build-server.example.com or 192.168.1.10:22"
            autoFocus
            {...register('host')}
          />
          {errors.host && (
            <p role="alert" className="mt-1 text-xs text-theme-danger">
              {errors.host.message}
            </p>
          )}
          <p className="mt-1 text-[11px] text-theme-muted">
            Use <code className="bg-theme-bg-soft px-0.5 rounded">host</code> or{' '}
            <code className="bg-theme-bg-soft px-0.5 rounded">host:port</code>.
            Defaults to port 22.
          </p>
        </div>

        <div>
          <label className={labelCls}>Description (optional)</label>
          <input
            className={inputCls}
            type="text"
            placeholder="e.g. Build server — 32 cores, 64 GB RAM"
            {...register('description')}
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
            {`ssh-keyscan -H ${hostValue.trim() || '<host>'} >> ~/.ssh/known_hosts`}
          </pre>
        </div>

        <div className="flex justify-end gap-2 pt-1">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-theme-line px-4 py-1.5 text-[13px] text-theme-text-secondary"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isSubmitting}
            className="rounded-md px-4 py-1.5 text-[13px] font-semibold text-white bg-theme-accent disabled:opacity-60"
          >
            {isSubmitting ? 'Adding…' : 'Add host'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
