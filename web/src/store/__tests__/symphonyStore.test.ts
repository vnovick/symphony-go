import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useSymphonyStore } from '../symphonyStore';

const EMPTY_SNAP = {
  running: [],
  paused: [],
  retrying: [],
  counts: { running: 0, retrying: 0, paused: 0 },
  generatedAt: '',
  maxConcurrentAgents: 3,
  rateLimits: null,
};

beforeEach(() => {
  useSymphonyStore.setState({
    snapshot: null,
    logs: [],
    sseConnected: false,
    selectedIdentifier: null,
    tokenSamples: [],
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('setSnapshot', () => {
  it('stores the snapshot', () => {
    useSymphonyStore.getState().setSnapshot(EMPTY_SNAP as never);
    expect(useSymphonyStore.getState().snapshot).toEqual(EMPTY_SNAP);
  });

  it('appends a token sample on every setSnapshot call', () => {
    const snap = { ...EMPTY_SNAP, running: [{ tokens: 500 }] };
    useSymphonyStore.getState().setSnapshot(snap as never);
    expect(useSymphonyStore.getState().tokenSamples).toHaveLength(1);
    expect(useSymphonyStore.getState().tokenSamples[0].totalTokens).toBe(500);
  });

  it('rolls the window when MAX_TOKEN_SAMPLES (60) is reached', () => {
    const snap = { ...EMPTY_SNAP, running: [{ tokens: 1 }] };
    for (let i = 0; i < 61; i++) {
      useSymphonyStore.getState().setSnapshot(snap as never);
    }
    expect(useSymphonyStore.getState().tokenSamples).toHaveLength(60);
  });
});

describe('appendLog', () => {
  it('appends log lines', () => {
    useSymphonyStore.getState().appendLog('line 1');
    useSymphonyStore.getState().appendLog('line 2');
    expect(useSymphonyStore.getState().logs).toEqual(['line 1', 'line 2']);
  });

  it('does not exceed MAX_LOG_LINES (500)', () => {
    for (let i = 0; i < 505; i++) {
      useSymphonyStore.getState().appendLog(`line ${String(i)}`);
    }
    expect(useSymphonyStore.getState().logs).toHaveLength(500);
    expect(useSymphonyStore.getState().logs.at(-1)).toBe('line 504');
  });
});

describe('clearLogs', () => {
  it('clears all logs', () => {
    useSymphonyStore.getState().appendLog('a');
    useSymphonyStore.getState().appendLog('b');
    useSymphonyStore.getState().clearLogs();
    expect(useSymphonyStore.getState().logs).toEqual([]);
  });
});

describe('setSseConnected', () => {
  it('sets sseConnected to true', () => {
    useSymphonyStore.getState().setSseConnected(true);
    expect(useSymphonyStore.getState().sseConnected).toBe(true);
  });

  it('sets sseConnected to false', () => {
    useSymphonyStore.setState({ sseConnected: true });
    useSymphonyStore.getState().setSseConnected(false);
    expect(useSymphonyStore.getState().sseConnected).toBe(false);
  });
});

describe('setSelectedIdentifier', () => {
  it('stores the identifier', () => {
    useSymphonyStore.getState().setSelectedIdentifier('ABC-1');
    expect(useSymphonyStore.getState().selectedIdentifier).toBe('ABC-1');
  });

  it('clears when null is passed', () => {
    useSymphonyStore.setState({ selectedIdentifier: 'ABC-1' });
    useSymphonyStore.getState().setSelectedIdentifier(null);
    expect(useSymphonyStore.getState().selectedIdentifier).toBeNull();
  });
});

describe('patchSnapshot', () => {
  it('merges partial fields into existing snapshot', () => {
    useSymphonyStore.setState({ snapshot: { ...EMPTY_SNAP, agentMode: '' } as never });
    useSymphonyStore.getState().patchSnapshot({ agentMode: 'teams' });
    expect(useSymphonyStore.getState().snapshot?.agentMode).toBe('teams');
  });

  it('merges multiple fields at once', () => {
    useSymphonyStore.setState({
      snapshot: { ...EMPTY_SNAP, activeStates: ['Todo'], terminalStates: ['Done'] } as never,
    });
    useSymphonyStore.getState().patchSnapshot({ activeStates: ['Todo', 'In Progress'] });
    const snap = useSymphonyStore.getState().snapshot as never as {
      activeStates: string[];
      terminalStates: string[];
    };
    expect(snap.activeStates).toEqual(['Todo', 'In Progress']);
    expect(snap.terminalStates).toEqual(['Done']); // unchanged
  });

  it('applies patch even when snapshot is null (optimistic pre-SSE update)', () => {
    useSymphonyStore.setState({ snapshot: null });
    useSymphonyStore.getState().patchSnapshot({ agentMode: 'teams' });
    // FE-7 fix: patch is applied to an empty base so optimistic updates are not dropped
    expect(useSymphonyStore.getState().snapshot?.agentMode).toBe('teams');
  });
});

describe('refreshSnapshot', () => {
  it('fetches /api/v1/state and updates snapshot', async () => {
    const mockSnap = {
      ...EMPTY_SNAP,
      running: [
        {
          identifier: 'ENG-1',
          state: 'running',
          turnCount: 0,
          tokens: 99,
          inputTokens: 0,
          outputTokens: 0,
          lastEvent: '',
          sessionId: '',
          workerHost: '',
          backend: 'claude',
          elapsedMs: 0,
          startedAt: '2024-01-01T00:00:00Z',
        },
      ],
    };
    global.fetch = vi
      .fn()
      .mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(mockSnap) });
    await useSymphonyStore.getState().refreshSnapshot();
    expect(useSymphonyStore.getState().snapshot).toEqual(mockSnap);
    expect(useSymphonyStore.getState().tokenSamples).toHaveLength(1);
    expect(useSymphonyStore.getState().tokenSamples[0].totalTokens).toBe(99);
  });

  it('does nothing when fetch fails', async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: false });
    await useSymphonyStore.getState().refreshSnapshot();
    expect(useSymphonyStore.getState().snapshot).toBeNull();
  });
});
