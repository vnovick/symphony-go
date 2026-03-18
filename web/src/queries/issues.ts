import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TrackerIssue } from '../types/symphony';

export const ISSUES_KEY = ['issues'] as const;

async function fetchIssues(): Promise<TrackerIssue[]> {
  const res = await fetch('/api/v1/issues');
  if (!res.ok) throw new Error(`fetch issues failed: ${String(res.status)}`);
  return res.json() as Promise<TrackerIssue[]>;
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

export function useUpdateIssueState() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ identifier, state }: { identifier: string; state: string }) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/state`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ state }),
      });
      if (!res.ok) throw new Error(`updateIssueState failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    },
  });
}

export function useSetIssueProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ identifier, profile }: { identifier: string; profile: string }) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/profile`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ profile }),
      });
      if (!res.ok) throw new Error(`setIssueProfile failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    },
  });
}

export function useCancelIssue() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (identifier: string) => {
      const res = await fetch(`/api/v1/issues/${encodeURIComponent(identifier)}/cancel`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`cancelIssue failed: ${String(res.status)}`);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
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
