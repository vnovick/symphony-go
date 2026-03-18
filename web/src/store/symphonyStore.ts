import { create } from 'zustand';
import type { StateSnapshot } from '../types/symphony';

const MAX_LOG_LINES = 500;
const MAX_TOKEN_SAMPLES = 60; // ~2 minute window at 1 sample/2s

export interface TokenSample {
  ts: number; // Date.now()
  totalTokens: number;
}

interface SymphonyState {
  snapshot: StateSnapshot | null;
  logs: string[];
  sseConnected: boolean;
  selectedIdentifier: string | null;
  tokenSamples: TokenSample[];
}

interface SymphonyActions {
  setSnapshot: (s: StateSnapshot) => void;
  appendLog: (line: string) => void;
  clearLogs: () => void;
  setSseConnected: (connected: boolean) => void;
  setSelectedIdentifier: (id: string | null) => void;
  patchSnapshot: (patch: Partial<StateSnapshot>) => void;
  refreshSnapshot: () => Promise<void>;
}

export type SymphonyStore = SymphonyState & SymphonyActions;

export const useSymphonyStore = create<SymphonyStore>((set) => ({
  snapshot: null,
  logs: [],
  sseConnected: false,
  selectedIdentifier: null,
  tokenSamples: [],

  setSnapshot: (snapshot) => {
    set((state) => {
      const totalTokens = snapshot.running.reduce((acc, r) => acc + r.tokens, 0);
      const sample: TokenSample = { ts: Date.now(), totalTokens };
      const prev = state.tokenSamples;
      // Drop the oldest entry when the window is full (FIFO rolling window).
      const next =
        prev.length >= MAX_TOKEN_SAMPLES ? [...prev.slice(1), sample] : [...prev, sample];
      return { snapshot, tokenSamples: next };
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

  patchSnapshot: (patch) => {
    set((state) => (state.snapshot ? { snapshot: { ...state.snapshot, ...patch } } : {}));
  },

  refreshSnapshot: async () => {
    try {
      const res = await fetch('/api/v1/state');
      if (!res.ok) return;
      const data = (await res.json()) as StateSnapshot;
      set((state) => {
        const totalTokens = data.running.reduce((acc, r) => acc + r.tokens, 0);
        const sample: TokenSample = { ts: Date.now(), totalTokens };
        const prev = state.tokenSamples;
        const next =
          prev.length >= MAX_TOKEN_SAMPLES ? [...prev.slice(1), sample] : [...prev, sample];
        return { snapshot: data, tokenSamples: next };
      });
    } catch {
      /* network error — silently ignore */
    }
  },
}));
