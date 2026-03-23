import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSymphonyStore } from '../store/symphonyStore';
import type { StateSnapshot, TrackerIssue } from '../types/symphony';
import { TrackerIssueSchema } from '../types/schemas';
import { z } from 'zod';

export const ISSUES_KEY = ['issues'] as const;

async function fetchIssues(): Promise<TrackerIssue[]> {
  const res = await fetch('/api/v1/issues');
  if (!res.ok) throw new Error(`fetch issues failed: ${String(res.status)}`);
  return z.array(TrackerIssueSchema).parse(await res.json());
}

export function useIssues() {
  return useQuery({
    queryKey: ISSUES_KEY,
    queryFn: fetchIssues,
    staleTime: 10_000,
    refetchInterval: 30_000,
  });
}

export function useInvalidateIssues() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
}

function optimisticPauseSnapshot(snapshot: StateSnapshot, identifier: string): StateSnapshot {
  const wasRunning = snapshot.running.some((row) => row.identifier === identifier);
  const alreadyPaused = snapshot.paused.includes(identifier);
  if (!wasRunning && alreadyPaused) {
    return snapshot;
  }
  return {
    ...snapshot,
    running: snapshot.running.filter((row) => row.identifier !== identifier),
    paused: alreadyPaused ? snapshot.paused : [...snapshot.paused, identifier],
    counts: {
      ...snapshot.counts,
      running: wasRunning ? Math.max(0, snapshot.counts.running - 1) : snapshot.counts.running,
      paused: !alreadyPaused && wasRunning ? snapshot.counts.paused + 1 : snapshot.counts.paused,
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
    onError: (_error, _vars, context) => {
      // Snap the card back to its original column if the API call fails.
      if (context?.prevIssues) {
        queryClient.setQueryData(ISSUES_KEY, context.prevIssues);
      }
    },
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
    onError: (_error, _vars, context) => {
      if (context?.prevIssues) {
        queryClient.setQueryData(ISSUES_KEY, context.prevIssues);
      }
    },
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
        useSymphonyStore.getState().setSnapshot(optimisticPauseSnapshot(prevSnapshot, identifier));
      }

      return { prevIssues, prevSnapshot };
    },
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/cancel`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`cancelIssue failed: ${String(res.status)}`);
    },
    onError: (_error, _identifier, context) => {
      if (context?.prevIssues) {
        queryClient.setQueryData(ISSUES_KEY, context.prevIssues);
      }
      if (context?.prevSnapshot) {
        useSymphonyStore.getState().setSnapshot(context.prevSnapshot);
      }
    },
    onSuccess: () => {
      setTimeout(() => {
        void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
        void useSymphonyStore.getState().refreshSnapshot();
      }, 1000);
    },
  });
}

export function useResumeIssue() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/resume`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`resumeIssue failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
      void useSymphonyStore.getState().refreshSnapshot();
    },
  });
}

export function useTerminateIssue() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/terminate`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`terminateIssue failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
      void useSymphonyStore.getState().refreshSnapshot();
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

export function useReanalyzeIssue() {
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/reanalyze`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`reanalyzeIssue failed: ${String(res.status)}`);
    },
  });
}
