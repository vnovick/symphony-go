import { useState, useEffect } from 'react';
import { useProjects } from '../../queries/projects';

interface Props {
  activeFilter: string[] | undefined;
  onSetFilter: (slugs: string[] | null) => Promise<boolean>;
}

export function ProjectFilterCard({ activeFilter, onSetFilter }: Props) {
  const { data: projects = [], isLoading, isError } = useProjects();
  const isDefaultMode = activeFilter === undefined;
  const [selected, setSelected] = useState<Set<string>>(
    new Set(isDefaultMode ? [] : activeFilter),
  );
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setSelected(new Set(isDefaultMode ? [] : activeFilter));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(activeFilter)]);

  const toggle = (slug: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(slug)) next.delete(slug);
      else next.add(slug);
      return next;
    });
  };

  const handleSave = async () => {
    setSaving(true);
    await onSetFilter([...selected]);
    setSaving(false);
  };

  const handleReset = async () => {
    setSaving(true);
    await onSetFilter(null);
    setSaving(false);
  };

  const selectAll = () => setSelected(new Set());
  const allSelected = selected.size === 0;

  return (
    <div
      className="overflow-hidden rounded-[var(--radius-md)]"
      style={{ border: '1px solid var(--line)', background: 'var(--bg-elevated)' }}
    >
      <div
        className="border-b px-5 py-4"
        style={{ borderColor: 'var(--line)', background: 'var(--panel-strong)' }}
      >
        <h2 className="text-sm font-semibold" style={{ color: 'var(--text)' }}>
          Project Filter
        </h2>
        <p className="mt-0.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
          Limit Symphony to specific Linear projects. Leave all unchecked to include every project.
        </p>
      </div>

      <div className="px-5 py-5 space-y-4">
        {isLoading && (
          <p className="text-sm" style={{ color: 'var(--muted)' }}>Loading projects…</p>
        )}

        {isError && (
          <p className="text-sm" style={{ color: 'var(--danger)' }}>
            Failed to load projects. Check that the server is running.
          </p>
        )}

        {!isLoading && !isError && (
          <>
            <div className="space-y-2">
              <label className="flex cursor-pointer items-center gap-2.5">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={selectAll}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: 'var(--accent)' }}
                />
                <span className="text-sm font-medium" style={{ color: 'var(--text)' }}>
                  All projects
                </span>
              </label>
              {projects.map((p) => (
                <label key={p.slug} className="flex cursor-pointer items-center gap-2.5 pl-1">
                  <input
                    type="checkbox"
                    checked={selected.has(p.slug)}
                    onChange={() => { toggle(p.slug); }}
                    className="h-4 w-4 rounded"
                    style={{ accentColor: 'var(--accent)' }}
                  />
                  <span className="text-sm" style={{ color: 'var(--text)' }}>{p.name}</span>
                  <span className="font-mono text-xs" style={{ color: 'var(--muted)' }}>
                    {p.slug}
                  </span>
                </label>
              ))}
            </div>

            {isDefaultMode && (
              <p className="text-xs" style={{ color: 'var(--muted)' }}>
                Currently using the WORKFLOW.md default project slug.
              </p>
            )}

            <div className="flex items-center gap-2 pt-1">
              <button
                onClick={handleSave}
                disabled={saving}
                className="rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
                style={{ background: 'var(--accent)' }}
              >
                {saving ? 'Saving…' : 'Save filter'}
              </button>
              {!isDefaultMode && (
                <button
                  onClick={handleReset}
                  disabled={saving}
                  className="rounded-[var(--radius-sm)] border px-4 py-2 text-sm font-medium transition-colors hover:opacity-80 disabled:opacity-50"
                  style={{ borderColor: 'var(--line)', color: 'var(--text-secondary)' }}
                >
                  Reset to default
                </button>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
