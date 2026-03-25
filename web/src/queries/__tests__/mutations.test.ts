import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  useUpdateIssueState,
  useCancelIssue,
  ISSUES_KEY,
} from '../issues';
import type { TrackerIssue } from '../../types/symphony';
import { useSymphonyStore } from '../../store/symphonyStore';
import { useToastStore } from '../../store/toastStore';

// ─── Helpers ──────────────────────────────────────────────────────────────────

function makeIssue(identifier: string, state = 'Todo'): TrackerIssue {
  return {
    identifier,
    title: `Title ${identifier}`,
    state,
    description: '',
    url: '',
    orchestratorState: 'idle',
    turnCount: 0,
    tokens: 0,
    elapsedMs: 0,
    lastMessage: '',
    error: '',
  };
}

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: qc }, children);
  };
}

function freshClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

// ─── Setup ────────────────────────────────────────────────────────────────────

beforeEach(() => {
  // Reset real stores to initial state before each test
  useSymphonyStore.setState({ snapshot: null, logs: [], sseConnected: false, selectedIdentifier: null, tokenSamples: [] });
  useToastStore.setState({ toasts: [], _timers: new Map() });
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ─── useUpdateIssueState ──────────────────────────────────────────────────────

describe('useUpdateIssueState', () => {
  it('applies optimistic update to the query cache immediately', async () => {
    const qc = freshClient();
    qc.setQueryData(ISSUES_KEY, [makeIssue('ABC-1', 'Todo')]);

    // Mutation fn hangs so we can inspect optimistic state before it resolves
    global.fetch = vi.fn().mockReturnValue(new Promise<Response>(() => {}));

    const { result } = renderHook(() => useUpdateIssueState(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ identifier: 'ABC-1', state: 'In Progress' });
    });

    // Wait one tick for onMutate's async steps (cancelQueries) to complete
    await new Promise((r) => setTimeout(r, 0));

    const cached = qc.getQueryData<TrackerIssue[]>(ISSUES_KEY);
    expect(cached?.[0].state).toBe('In Progress');
  });

  it('rolls back the cache when the API call fails', async () => {
    const qc = freshClient();
    qc.setQueryData(ISSUES_KEY, [makeIssue('ABC-1', 'Todo')]);

    global.fetch = vi.fn().mockRejectedValue(new Error('network error'));

    const { result } = renderHook(() => useUpdateIssueState(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ identifier: 'ABC-1', state: 'In Progress' });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    const cached = qc.getQueryData<TrackerIssue[]>(ISSUES_KEY);
    expect(cached?.[0].state).toBe('Todo');
  });

  it('adds a toast when the API call fails', async () => {
    const qc = freshClient();
    qc.setQueryData(ISSUES_KEY, [makeIssue('ABC-1', 'Todo')]);

    global.fetch = vi.fn().mockRejectedValue(new Error('server exploded'));

    const { result } = renderHook(() => useUpdateIssueState(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ identifier: 'ABC-1', state: 'Done' });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    const toasts = useToastStore.getState().toasts;
    expect(toasts.length).toBeGreaterThan(0);
    expect(toasts[0].message).toMatch(/server exploded/i);
  });
});

// ─── useCancelIssue ───────────────────────────────────────────────────────────

describe('useCancelIssue', () => {
  it('applies optimistic patch to the snapshot on mutate', async () => {
    const qc = freshClient();

    const baseSnapshot = {
      generatedAt: new Date().toISOString(),
      orchestratorState: 'running',
      running: [
        {
          identifier: 'ABC-2',
          state: 'In Progress',
          turnCount: 1,
          tokens: 100,
          inputTokens: 50,
          outputTokens: 50,
          elapsedMs: 1000,
          startedAt: new Date().toISOString(),
          sessionId: 's1',
        },
      ],
      retrying: [],
      paused: [],
      pausedWithPR: [],
      counts: { running: 1, retrying: 0, paused: 0 },
      maxConcurrentAgents: 5,
      rateLimits: null,
    } as unknown as import('../../types/symphony').StateSnapshot;

    useSymphonyStore.setState({ snapshot: baseSnapshot });

    const patchSpy = vi.spyOn(useSymphonyStore.getState(), 'patchSnapshot');

    global.fetch = vi.fn().mockReturnValue(new Promise<Response>(() => {}));

    const { result } = renderHook(() => useCancelIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate('ABC-2');
    });

    await new Promise((r) => setTimeout(r, 0));

    expect(patchSpy).toHaveBeenCalled();
    const patchArg = patchSpy.mock.calls[0][0];
    expect(patchArg.paused).toContain('ABC-2');
  });

  it('rolls back both cache and snapshot on API error', async () => {
    const qc = freshClient();
    qc.setQueryData(ISSUES_KEY, [makeIssue('ABC-2', 'In Progress')]);

    const baseSnapshot = {
      generatedAt: new Date().toISOString(),
      orchestratorState: 'running',
      running: [
        {
          identifier: 'ABC-2',
          state: 'In Progress',
          turnCount: 0,
          tokens: 0,
          inputTokens: 0,
          outputTokens: 0,
          elapsedMs: 0,
          startedAt: new Date().toISOString(),
          sessionId: 's1',
        },
      ],
      retrying: [],
      paused: [],
      pausedWithPR: [],
      counts: { running: 1, retrying: 0, paused: 0 },
      maxConcurrentAgents: 5,
      rateLimits: null,
    } as unknown as import('../../types/symphony').StateSnapshot;

    useSymphonyStore.setState({ snapshot: baseSnapshot });

    global.fetch = vi.fn().mockRejectedValue(new Error('cancel failed'));

    const { result } = renderHook(() => useCancelIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate('ABC-2');
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    // Cache should be restored
    const cached = qc.getQueryData<TrackerIssue[]>(ISSUES_KEY);
    expect(cached?.[0].orchestratorState).toBe('idle');

    // A toast should have been shown
    const toasts = useToastStore.getState().toasts;
    expect(toasts.length).toBeGreaterThan(0);
  });
});
