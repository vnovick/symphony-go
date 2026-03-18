import { useSymphonyStore } from '../../store/symphonyStore';

function fmtDueAt(dueAtStr: string): string {
  const diff = Math.round((new Date(dueAtStr).getTime() - Date.now()) / 1000);
  if (diff <= 0) return 'now';
  if (diff < 60) return `in ${String(diff)}s`;
  return `in ${String(Math.ceil(diff / 60))}m`;
}

export default function RetryQueueTable() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const retrying = snapshot?.retrying ?? [];
  if (retrying.length === 0) return null;

  return (
    <div className="rounded-2xl border border-amber-200 bg-amber-50 dark:border-amber-800/50 dark:bg-amber-900/10">
      <div className="border-b border-amber-200 px-6 py-4 dark:border-amber-800/50">
        <h3 className="text-base font-semibold text-amber-900 dark:text-amber-300">
          Retry Queue
          <span className="ml-2 inline-flex items-center rounded-full bg-amber-200 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-800/50 dark:text-amber-300">
            {retrying.length}
          </span>
          <span className="ml-2 text-sm font-normal text-amber-700 dark:text-amber-400">
            — click a row to view logs
          </span>
        </h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr>
              {['Identifier', 'Attempt', 'Due', 'Last Error'].map((h) => (
                <th
                  key={h}
                  className="px-4 py-3 text-left text-xs font-medium tracking-wider text-amber-700 uppercase dark:text-amber-400"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-amber-100 dark:divide-amber-800/30">
            {retrying.map((row) => (
              <tr
                key={row.identifier}
                className="cursor-pointer hover:bg-amber-100/60 dark:hover:bg-amber-800/20"
                onClick={() => {
                  setSelectedIdentifier(row.identifier);
                }}
              >
                <td className="px-4 py-3 font-mono text-sm font-medium text-gray-900 dark:text-white">
                  {row.identifier}
                </td>
                <td className="px-4 py-3 text-amber-800 dark:text-amber-300">#{row.attempt}</td>
                <td className="px-4 py-3 font-mono text-xs text-amber-700 dark:text-amber-400">
                  {fmtDueAt(row.dueAt)}
                </td>
                <td
                  className="max-w-sm truncate px-4 py-3 text-xs text-gray-500 dark:text-gray-400"
                  title={row.error}
                >
                  {row.error || '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
