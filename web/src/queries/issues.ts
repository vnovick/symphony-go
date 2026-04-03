import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import { useSymphonyStore } from '../store/symphonyStore';
import { useToastStore } from '../store/toastStore';
import type { StateSnapshot, TrackerIssue } from '../types/schemas';
import { TrackerIssueSchema } from '../types/schemas';
import { z } from 'zod';

export const ISSUES_KEY = ['issues'] as const;
export const ISSUE_KEY = (identifier: string) => ['issue', identifier] as const;

type RollbackContext =
  | { prevIssues?: TrackerIssue[]; prevSnapshot?: StateSnapshot }
  | undefined;

/** Extracts a user-facing message from an unknown error and shows it as a toast. */
function toastApiError(err: unknown, fallback = 'Action failed — please try again.'): void {
  const message = err instanceof Error ? err.message : fallback;
  useToastStore.getState().addToast(message);
}

/**
 * Returns an `onError` handler that rolls back optimistic query/snapshot updates
 * and surfaces the error to the user via a toast notification.
 * Used by all issue mutations that apply optimistic updates.
 */
function makeRollbackHandler(queryClient: QueryClient) {
  return (_error: unknown, _vars: unknown, context: RollbackContext) => {
    if (context?.prevIssues) queryClient.setQueryData(ISSUES_KEY, context.prevIssues);
    if (context?.prevSnapshot) useSymphonyStore.getState().setSnapshot(context.prevSnapshot);
    toastApiError(_error);
  };
}

async function fetchIssues(): Promise<TrackerIssue[]> {
  const res = await fetch('/api/v1/issues');
  if (!res.ok) throw new Error(`fetch issues failed: ${String(res.status)}`);
  return z.array(TrackerIssueSchema).parse(await res.json());
}

export function useIssues() {
  return useQuery({
    queryKey: ISSUES_KEY,
    queryFn: fetchIssues,
    staleTime: 5_000,
  });
}

export function useInvalidateIssues() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
}

export function useIssue(identifier: string) {
  return useQuery({
    queryKey: ISSUE_KEY(identifier),
    queryFn: async () => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}`);
      if (!res.ok) throw new Error(`fetch issue failed: ${String(res.status)}`);
      return TrackerIssueSchema.parse(await res.json());
    },
    enabled: identifier !== '',
    staleTime: 0,
  });
}

function optimisticPauseSnapshot(snapshot: StateSnapshot, identifier: string): StateSnapshot {
  const wasRunning = snapshot.running.some((row) => row.identifier === identifier);
  const wasRetrying = snapshot.retrying.some((row) => row.identifier === identifier);
  const alreadyPaused = snapshot.paused.includes(identifier);
  if (!wasRunning && !wasRetrying && alreadyPaused) {
    return snapshot;
  }
  // Note: `pausedWithPR` is intentionally not updated here. The PR URL is not
  // known client-side at optimistic-update time; it will appear once the next
  // real snapshot arrives from the server.
  return {
    ...snapshot,
    running: snapshot.running.filter((row) => row.identifier !== identifier),
    retrying: snapshot.retrying.filter((row) => row.identifier !== identifier),
    paused: alreadyPaused ? snapshot.paused : [...snapshot.paused, identifier],
    counts: {
      ...snapshot.counts,
      running: wasRunning ? Math.max(0, snapshot.counts.running - 1) : snapshot.counts.running,
      paused: !alreadyPaused && (wasRunning || wasRetrying) ? snapshot.counts.paused + 1 : snapshot.counts.paused,
    },
  };
}

export function useUpdateIssueState() {
  const queryClient = useQueryClient();
  return useMutation({
    onMutate: async ({ identifier, state }: { identifier: string; state: string }) => {
      // Cancel any in-flight refetches so they don't overwrite the optimistic update.
      await queryClient.cancelQueries({ queryKey: ISSUES_KEY });
      const prevIssues = queryClient.getQueryData<TrackerIssue[]>(ISSUES_KEY);

      if (prevIssues) {
        queryClient.setQueryData<TrackerIssue[]>(
          ISSUES_KEY,
          prevIssues.map((issue) =>
            issue.identifier === identifier ? { ...issue, state } : issue,
          ),
        );
      }

      return { prevIssues };
    },
    mutationFn: async ({ identifier, state }: { identifier: string; state: string }) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/state`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ state }),
      });
      if (!res.ok) throw new Error(`updateIssueState failed: ${String(res.status)}`);
    },
    onError: makeRollbackHandler(queryClient),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    },
  });
}

export function useSetIssueProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    onMutate: async ({ identifier, profile }: { identifier: string; profile: string }) => {
      await queryClient.cancelQueries({ queryKey: ISSUES_KEY });
      const prevIssues = queryClient.getQueryData<TrackerIssue[]>(ISSUES_KEY);
      if (prevIssues) {
        queryClient.setQueryData<TrackerIssue[]>(
          ISSUES_KEY,
          prevIssues.map((i) =>
            i.identifier === identifier ? { ...i, agentProfile: profile || undefined } : i,
          ),
        );
      }
      return { prevIssues };
    },
    mutationFn: async ({ identifier, profile }: { identifier: string; profile: string }) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/profile`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ profile }),
      });
      if (!res.ok) throw new Error(`setIssueProfile failed: ${String(res.status)}`);
    },
    onError: makeRollbackHandler(queryClient),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    },
  });
}

export function useSetIssueBackend() {
  const queryClient = useQueryClient();
  return useMutation({
    onMutate: async ({ identifier, backend }: { identifier: string; backend: string }) => {
      await queryClient.cancelQueries({ queryKey: ISSUES_KEY });
      const prevIssues = queryClient.getQueryData<TrackerIssue[]>(ISSUES_KEY);
      if (prevIssues) {
        queryClient.setQueryData<TrackerIssue[]>(
          ISSUES_KEY,
          prevIssues.map((i) =>
            i.identifier === identifier ? { ...i, agentBackend: backend || undefined } : i,
          ),
        );
      }
      return { prevIssues };
    },
    mutationFn: async ({ identifier, backend }: { identifier: string; backend: string }) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/backend`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ backend }),
      });
      if (!res.ok) throw new Error(`setIssueBackend failed: ${String(res.status)}`);
    },
    onError: makeRollbackHandler(queryClient),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    },
  });
}

export function useCancelIssue() {
  const queryClient = useQueryClient();
  return useMutation({
    onMutate: async (identifier: string) => {
      await queryClient.cancelQueries({ queryKey: ISSUES_KEY });
      const prevIssues = queryClient.getQueryData<TrackerIssue[]>(ISSUES_KEY);
      const prevSnapshot = useSymphonyStore.getState().snapshot;

      if (prevIssues) {
        queryClient.setQueryData<TrackerIssue[]>(
          ISSUES_KEY,
          prevIssues.map((issue) =>
            issue.identifier === identifier ? { ...issue, orchestratorState: 'paused' } : issue,
          ),
        );
      }
      if (prevSnapshot) {
        const updated = optimisticPauseSnapshot(prevSnapshot, identifier);
        useSymphonyStore.getState().patchSnapshot({
          running: updated.running,
          counts: updated.counts,
          paused: updated.paused,
        });
      }

      return { prevIssues, prevSnapshot: prevSnapshot ?? undefined };
    },
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/cancel`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`cancelIssue failed: ${String(res.status)}`);
    },
    onError: makeRollbackHandler(queryClient),
    onSuccess: () => {
      // SSE + useSnapshotInvalidation handle both snapshot and issue list updates.
      // refreshSnapshot ensures immediate consistency if SSE is lagging.
      void useSymphonyStore.getState().refreshSnapshot();
    },
  });
}

export function useResumeIssue() {
  const queryClient = useQueryClient();
  return useMutation({
    onMutate: async (identifier: string) => {
      await queryClient.cancelQueries({ queryKey: ISSUES_KEY });
      const prevIssues = queryClient.getQueryData<TrackerIssue[]>(ISSUES_KEY);
      const prevSnapshot = useSymphonyStore.getState().snapshot;

      if (prevIssues) {
        queryClient.setQueryData<TrackerIssue[]>(
          ISSUES_KEY,
          prevIssues.map((issue) =>
            issue.identifier === identifier
              ? { ...issue, orchestratorState: 'running' }
              : issue,
          ),
        );
      }
      if (prevSnapshot) {
        const wasInPaused = prevSnapshot.paused.includes(identifier);
        const updatedPaused = prevSnapshot.paused.filter((id) => id !== identifier);
        // Use the issue's real tracker state for the badge — 'running' is an
        // orchestrator state, not a tracker state, so the badge would render
        // incorrectly until the next SSE update (FE-R10-2).
        const issueTrackerState = prevIssues?.find((i) => i.identifier === identifier)?.state ?? '';
        const optimisticRow = {
          identifier,
          state: issueTrackerState,
          turnCount: 0,
          tokens: 0,
          inputTokens: 0,
          outputTokens: 0,
          elapsedMs: 0,
          startedAt: new Date().toISOString(),
          sessionId: '',
        };
        useSymphonyStore.getState().patchSnapshot({
          paused: updatedPaused,
          running: wasInPaused
            ? [...prevSnapshot.running, optimisticRow]
            : prevSnapshot.running,
          counts: {
            ...prevSnapshot.counts,
            paused: wasInPaused
              ? Math.max(0, prevSnapshot.counts.paused - 1)
              : prevSnapshot.counts.paused,
            running: wasInPaused
              ? prevSnapshot.counts.running + 1
              : prevSnapshot.counts.running,
          },
        });
      }

      return { prevIssues, prevSnapshot: prevSnapshot ?? undefined };
    },
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/resume`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`resumeIssue failed: ${String(res.status)}`);
    },
    onError: makeRollbackHandler(queryClient),
    onSuccess: () => {
      void useSymphonyStore.getState().refreshSnapshot();
    },
  });
}

export function useTerminateIssue() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/terminate`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`terminateIssue failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void useSymphonyStore.getState().refreshSnapshot();
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Terminate failed — please try again.');
    },
  });
}

export function useTriggerAIReview() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/ai-review`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`triggerAIReview failed: ${String(res.status)}`);
    },
    onError: (err: unknown) => {
      toastApiError(err, 'AI review trigger failed — please try again.');
    },
  });
}

export function useClearIssueLogs() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/logs`, {
        method: 'DELETE',
      });
      if (!res.ok) throw new Error(`clearIssueLogs failed: ${String(res.status)}`);
    },
  });
}

export function useClearAllLogs() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const res = await fetch('/api/v1/logs', { method: 'DELETE' });
      if (!res.ok) throw new Error(`clearAllLogs failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['logs'] });
      void queryClient.invalidateQueries({ queryKey: ['sublogs'] });
      void queryClient.invalidateQueries({ queryKey: ['log-identifiers'] });
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Clear all logs failed — please try again.');
    },
  });
}

export function useClearAllWorkspaces() {
  return useMutation({
    mutationFn: async () => {
      const res = await fetch('/api/v1/workspaces', { method: 'DELETE' });
      if (!res.ok) throw new Error(`clearAllWorkspaces failed: ${String(res.status)}`);
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Reset workspaces failed — please try again.');
    },
  });
}

export function useClearIssueSubLogs() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/sublogs`, {
        method: 'DELETE',
      });
      if (!res.ok) throw new Error(`clearIssueSubLogs failed: ${String(res.status)}`);
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Clear session logs failed — please try again.');
    },
  });
}

export function useProvideInput() {
  return useMutation({
    mutationFn: async ({ identifier, message }: { identifier: string; message: string }) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/provide-input`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message }),
      });
      if (!res.ok) throw new Error(`provideInput failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void useSymphonyStore.getState().refreshSnapshot();
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Failed to send input to agent.');
    },
  });
}

export function useDismissInput() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/dismiss-input`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`dismissInput failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void useSymphonyStore.getState().refreshSnapshot();
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Failed to dismiss input request.');
    },
  });
}

export function useReanalyzeIssue() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/reanalyze`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`reanalyzeIssue failed: ${String(res.status)}`);
    },
    onError: (err: unknown) => {
      toastApiError(err, 'Re-analysis failed — please try again.');
    },
  });
}
