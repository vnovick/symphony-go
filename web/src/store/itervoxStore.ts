import { create } from 'zustand';
import type { StateSnapshot } from '../types/schemas';
import { StateSnapshotSchema } from '../types/schemas';
import { authedFetch } from '../auth/authedFetch';
import { UnauthorizedError } from '../auth/UnauthorizedError';

const MAX_LOG_LINES = 500;
const MAX_TOKEN_SAMPLES = 60; // ~2 minute window at 1 sample/2s

export interface TokenSample {
  ts: number; // Date.now()
  totalTokens: number;
}

// appendTokenSample adds a new sample derived from snapshot to the rolling
// window, evicting the oldest entry when the window is full.
// Returns the previous array unchanged when totalTokens hasn't changed to
// avoid unnecessary store updates.
export function appendTokenSample(prev: TokenSample[], snapshot: StateSnapshot): TokenSample[] {
  const totalTokens = snapshot.running.reduce((acc, r) => acc + r.tokens, 0);
  // Skip if last sample has same totalTokens — avoid unnecessary store updates.
  if (prev.length > 0 && prev[prev.length - 1].totalTokens === totalTokens) {
    return prev;
  }
  const sample: TokenSample = { ts: Date.now(), totalTokens };
  return prev.length >= MAX_TOKEN_SAMPLES ? [...prev.slice(1), sample] : [...prev, sample];
}

interface ItervoxState {
  snapshot: StateSnapshot | null;
  logs: string[];
  sseConnected: boolean;
  selectedIdentifier: string | null;
  /** Cross-page active issue — persists across Timeline/Logs/Dashboard navigation */
  activeIssueId: string | null;
  tokenSamples: TokenSample[];
}

interface ItervoxActions {
  setSnapshot: (s: StateSnapshot) => void;
  appendLog: (line: string) => void;
  clearLogs: () => void;
  setSseConnected: (connected: boolean) => void;
  setSelectedIdentifier: (id: string | null) => void;
  setActiveIssueId: (id: string | null) => void;
  patchSnapshot: (patch: Partial<StateSnapshot>) => void;
  refreshSnapshot: () => Promise<void>;
}

export type ItervoxStore = ItervoxState & ItervoxActions;

export const useItervoxStore = create<ItervoxStore>((set) => ({
  snapshot: null,
  logs: [],
  sseConnected: false,
  selectedIdentifier: null,
  activeIssueId: null,
  tokenSamples: [],

  setSnapshot: (snapshot) => {
    set((state) => {
      const tokenSamples = appendTokenSample(state.tokenSamples, snapshot);
      return tokenSamples === state.tokenSamples ? { snapshot } : { snapshot, tokenSamples };
    });
  },

  appendLog: (line) => {
    set((state) => ({
      logs:
        state.logs.length >= MAX_LOG_LINES
          ? [...state.logs.slice(state.logs.length - MAX_LOG_LINES + 1), line]
          : [...state.logs, line],
    }));
  },

  clearLogs: () => {
    set({ logs: [] });
  },
  setSseConnected: (sseConnected) => {
    set({ sseConnected });
  },
  setSelectedIdentifier: (selectedIdentifier) => {
    set({ selectedIdentifier });
  },
  setActiveIssueId: (activeIssueId) => {
    set({ activeIssueId });
  },

  patchSnapshot: (patch) => {
    set((state) => ({
      snapshot: { ...(state.snapshot ?? {}), ...patch } as StateSnapshot,
    }));
  },

  refreshSnapshot: async () => {
    try {
      const res = await authedFetch('/api/v1/state');
      if (!res.ok) return;
      const data: StateSnapshot = StateSnapshotSchema.parse(await res.json());
      set((state) => ({
        snapshot: data,
        tokenSamples: appendTokenSample(state.tokenSamples, data),
      }));
    } catch (err) {
      if (err instanceof UnauthorizedError) return; // AuthGate will handle.
      if (import.meta.env.DEV) {
        console.warn('[itervox] refreshSnapshot failed — state may be stale', err);
      }
    }
  },
}));
