import { useState, useEffect, useRef } from 'react';
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

  // Sync local selection when server state changes (new array reference with
  // different content). Uses a ref to compare by value instead of reference.
  const prevFilterRef = useRef(activeFilter);
  useEffect(() => {
    const prev = prevFilterRef.current;
    const changed =
      prev === activeFilter ? false
      : prev === undefined || activeFilter === undefined ? true
      : prev.length !== activeFilter.length || prev.some((s, i) => s !== activeFilter[i]);
    if (changed) {
      setSelected(new Set(activeFilter ?? []));
    }
    prevFilterRef.current = activeFilter;
  }, [activeFilter]);

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
      className="overflow-hidden rounded-[var(--radius-md)] border border-theme-line bg-theme-bg-elevated"
    >
      <div
        className="border-b px-5 py-4 border-theme-line bg-theme-panel-strong"
      >
        <h2 className="text-sm font-semibold text-theme-text">
          Project Filter
        </h2>
        <p className="mt-0.5 text-xs text-theme-text-secondary">
          Limit Symphony to specific Linear projects. Leave all unchecked to include every project.
        </p>
      </div>

      <div className="px-5 py-5 space-y-4">
        {isLoading && (
          <p className="text-sm text-theme-muted">Loading projects…</p>
        )}

        {isError && (
          <p className="text-sm text-theme-danger">
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
                <span className="text-sm font-medium text-theme-text">
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
                  <span className="text-sm text-theme-text">{p.name}</span>
                  <span className="font-mono text-xs text-theme-muted">
                    {p.slug}
                  </span>
                </label>
              ))}
            </div>

            {isDefaultMode && (
              <p className="text-xs text-theme-muted">
                Currently using the WORKFLOW.md default project slug.
              </p>
            )}

            <div className="flex items-center gap-2 pt-1">
              <button
                onClick={handleSave}
                disabled={saving}
                className="rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50 bg-theme-accent"
              >
                {saving ? 'Saving…' : 'Save filter'}
              </button>
              {!isDefaultMode && (
                <button
                  onClick={handleReset}
                  disabled={saving}
                  className="rounded-[var(--radius-sm)] border px-4 py-2 text-sm font-medium transition-colors hover:opacity-80 disabled:opacity-50 border-theme-line text-theme-text-secondary"
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
