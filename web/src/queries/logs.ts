import { useQuery } from '@tanstack/react-query';
import type { IssueLogEntry } from '../types/symphony';

export const logsKey = (identifier: string) => ['logs', identifier] as const;

async function fetchIssueLogs(identifier: string): Promise<IssueLogEntry[]> {
  const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/logs`);
  if (!res.ok) throw new Error(`fetch logs failed: ${String(res.status)}`);
  return res.json() as Promise<IssueLogEntry[]>;
}

export function useIssueLogs(identifier: string, isLive: boolean) {
  return useQuery({
    queryKey: logsKey(identifier),
    queryFn: () => fetchIssueLogs(identifier),
    enabled: !!identifier,
    refetchInterval: isLive ? 2000 : 30_000,
    staleTime: 0,
  });
}
