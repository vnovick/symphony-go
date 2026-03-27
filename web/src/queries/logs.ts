import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { IssueLogEntry } from '../types/schemas';
import { IssueLogEntrySchema } from '../types/schemas';
import { z } from 'zod';

export const logsKey = (identifier: string) => ['logs', identifier] as const;
export const sublogsKey = (identifier: string) => ['sublogs', identifier] as const;
export const logIdentifiersKey = () => ['log-identifiers'] as const;

async function fetchLogIdentifiers(): Promise<string[]> {
  const res = await fetch('/api/v1/logs/identifiers');
  if (!res.ok) throw new Error(`fetch log identifiers failed: ${String(res.status)}`);
  return z.array(z.string()).parse(await res.json());
}

async function fetchIssueLogs(identifier: string): Promise<IssueLogEntry[]> {
  const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/logs`);
  if (!res.ok) throw new Error(`fetch logs failed: ${String(res.status)}`);
  return z.array(IssueLogEntrySchema).parse(await res.json());
}

async function fetchSubLogs(identifier: string): Promise<IssueLogEntry[]> {
  const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/sublogs`);
  if (!res.ok) throw new Error(`fetch sublogs failed: ${String(res.status)}`);
  return z.array(IssueLogEntrySchema).parse(await res.json());
}

/**
 * Fetches issue log entries.
 *
 * - isLive=true: uses SSE (/api/v1/issues/{id}/log-stream) — push-based, no polling.
 * - isLive=false: one-shot TanStack Query fetch with 30s stale time.
 *
 * API is identical for all callers regardless of mode.
 */
export function useIssueLogs(identifier: string, isLive: boolean) {
  // SSE state — always declared (rules of hooks), activated only when isLive
  const [sseData, setSseData] = useState<IssueLogEntry[]>([]);
  const [sseLoading, setSseLoading] = useState(false);
  const [sseError, setSseError] = useState(false);
  // Ref to prevent stale-closure accumulation when identifier changes
  const identifierRef = useRef(identifier);
  identifierRef.current = identifier;

  useEffect(() => {
    if (!isLive || !identifier) return;

    setSseData([]);
    setSseLoading(true);
    setSseError(false);

    const es = new EventSource(`/api/v1/issues/${encodeURIComponent(identifier)}/log-stream`);

    es.onopen = () => {
      // Connection confirmed; initial batch events follow immediately
      setSseLoading(false);
    };

    es.addEventListener('log', (e: MessageEvent) => {
      try {
        const entry = IssueLogEntrySchema.parse(JSON.parse(String(e.data)) as unknown);
        // Guard: ignore late events from a previous identifier's stream
        if (identifierRef.current === identifier) {
          setSseData((prev) => [...prev, entry]);
        }
      } catch {
        // malformed event — skip
      }
    });

    es.onerror = () => {
      setSseError(true);
      setSseLoading(false);
      es.close();
    };

    return () => {
      es.close();
    };
  }, [identifier, isLive]);

  // One-shot query — disabled when isLive to avoid redundant HTTP fetches
  const { data: queryData, isLoading: queryLoading, isError: queryError } = useQuery({
    queryKey: logsKey(identifier),
    queryFn: () => fetchIssueLogs(identifier),
    enabled: !!identifier && !isLive,
    staleTime: 15_000,
  });

  if (isLive) {
    return { data: sseData, isLoading: sseLoading, isError: sseError };
  }
  return { data: queryData ?? [], isLoading: queryLoading, isError: queryError };
}

/**
 * Fetches full session logs written by Claude Code to CLAUDE_CODE_LOG_DIR.
 * Covers all subagents, not just the top-level orchestrator log buffer.
 * For live sessions this is polled every 5s; for completed sessions fetched once.
 */
export function useSubagentLogs(identifier: string, isLive: boolean) {
  const { data, isLoading, isError } = useQuery({
    queryKey: sublogsKey(identifier),
    queryFn: () => fetchSubLogs(identifier),
    enabled: !!identifier,
    refetchInterval: isLive ? 5000 : false,
    staleTime: isLive ? 3000 : Infinity,
  });
  return { data, isLoading, isError };
}

/**
 * Returns the list of issue identifiers that have log data on the server
 * (either in-memory or persisted to disk). Use this for the Logs sidebar
 * instead of the full issue list from the tracker.
 */
export function useLogIdentifiers() {
  const { data = [] } = useQuery({
    queryKey: logIdentifiersKey(),
    queryFn: fetchLogIdentifiers,
    staleTime: 10_000,
  });
  return data;
}
