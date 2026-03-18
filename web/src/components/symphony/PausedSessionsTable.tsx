import { useSymphonyStore } from '../../store/symphonyStore';
import { useResumeIssue } from '../../queries/issues';

export default function PausedSessionsTable() {
  const snapshot = useSymphonyStore((s) => s.snapshot);
  const resumeIssueMutation = useResumeIssue();
  const setSelectedIdentifier = useSymphonyStore((s) => s.setSelectedIdentifier);
  const paused = snapshot?.paused ?? [];

  if (paused.length === 0) return null;

  return (
    <div className="rounded-2xl border border-red-200 bg-red-50 dark:border-red-800/50 dark:bg-red-900/10">
      <div className="border-b border-red-200 px-6 py-4 dark:border-red-800/50">
        <h3 className="text-base font-semibold text-red-900 dark:text-red-300">
          Paused
          <span className="ml-2 inline-flex items-center rounded-full bg-red-200 px-2 py-0.5 text-xs font-medium text-red-800 dark:bg-red-800/50 dark:text-red-300">
            {paused.length}
          </span>
          <span className="ml-2 text-sm font-normal text-red-600 dark:text-red-400">
            — killed by user, will not auto-retry
          </span>
        </h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr>
              {['Identifier', 'Actions'].map((h) => (
                <th
                  key={h}
                  className="px-4 py-3 text-left text-xs font-medium tracking-wider text-red-700 uppercase dark:text-red-400"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-red-100 dark:divide-red-800/30">
            {paused.map((identifier) => (
              <tr
                key={identifier}
                className="cursor-pointer hover:bg-red-100/60 dark:hover:bg-red-800/20"
                onClick={() => {
                  setSelectedIdentifier(identifier);
                }}
              >
                <td className="px-4 py-3 font-mono text-sm font-medium text-gray-900 dark:text-white">
                  {identifier}
                </td>
                <td
                  className="px-4 py-3"
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                >
                  <button
                    onClick={() => {
                      resumeIssueMutation.mutate(identifier);
                    }}
                    className="rounded border border-green-300 px-2 py-1 text-xs text-green-700 hover:bg-green-50 dark:border-green-700 dark:text-green-400 dark:hover:bg-green-900/20"
                  >
                    ▶ Resume
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
