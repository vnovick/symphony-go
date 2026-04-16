import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import AppHeader from '../AppHeader';
import { useItervoxStore } from '../../store/itervoxStore';

vi.mock('zustand/react/shallow', () => ({
  useShallow: (fn: unknown) => fn,
}));

vi.mock('../../store/itervoxStore', () => ({
  useItervoxStore: vi.fn(),
}));

const mockUseItervoxStore = vi.mocked(useItervoxStore);

function setupStore(snapshotOverride: Record<string, unknown> = {}) {
  const state = {
    sseConnected: true,
    snapshot: {
      running: [],
      paused: [],
      retrying: [],
      inputRequired: [],
      maxConcurrentAgents: 3,
      agentMode: '',
      ...snapshotOverride,
    },
  };
  mockUseItervoxStore.mockImplementation((selector: (s: typeof state) => unknown) =>
    selector(state),
  );
}

describe('AppHeader', () => {
  it('shows pending resume instead of idle when a reply is queued', () => {
    setupStore({
      inputRequired: [
        {
          identifier: 'ENG-1',
          sessionId: 'session-1',
          state: 'pending_input_resume',
          context: 'Reply received, waiting to resume.',
          queuedAt: '2026-04-15T00:00:00Z',
        },
      ],
    });

    render(<AppHeader />);

    expect(screen.getByText('reply received')).toBeInTheDocument();
    expect(screen.getByText('1 resuming')).toBeInTheDocument();
  });

  it('shows input required instead of idle when human input is still needed', () => {
    setupStore({
      inputRequired: [
        {
          identifier: 'ENG-2',
          sessionId: 'session-2',
          state: 'input_required',
          context: 'Need approval',
          queuedAt: '2026-04-15T00:00:00Z',
        },
      ],
    });

    render(<AppHeader />);

    expect(screen.getByText('input required')).toBeInTheDocument();
    expect(screen.getByText('1 need input')).toBeInTheDocument();
  });
});
