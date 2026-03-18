import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import StatusStrip from '../StatusStrip';

vi.mock('../../../store/symphonyStore', () => ({
  useSymphonyStore: vi.fn(),
}));

import { useSymphonyStore } from '../../../store/symphonyStore';

function mockStore(overrides = {}) {
  const defaults = {
    snapshot: {
      running: [{ tokens: 100 }, { tokens: 200 }],
      paused: [],
      retrying: [],
      maxConcurrentAgents: 5,
      agentMode: '',
    },
    refreshSnapshot: vi.fn(),
  };
  const store = { ...defaults, ...overrides };
  (useSymphonyStore as unknown as ReturnType<typeof vi.fn>).mockImplementation(
    (sel: (s: typeof store) => unknown) => sel(store),
  );
  return store;
}

describe('StatusStrip', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders running count', () => {
    mockStore();
    render(<StatusStrip />);
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
  });

  it('does not show paused badge when paused is empty', () => {
    mockStore();
    render(<StatusStrip />);
    expect(screen.queryByText(/paused/)).not.toBeInTheDocument();
  });

  it('shows paused count badge when there are paused agents', () => {
    mockStore({
      snapshot: {
        running: [],
        paused: ['ABC-1', 'ABC-2'],
        retrying: [],
        maxConcurrentAgents: 5,
        agentMode: '',
      },
    });
    render(<StatusStrip />);
    expect(screen.getByText('2 paused')).toBeInTheDocument();
  });

  it('shows retrying badge when retrying', () => {
    mockStore({
      snapshot: {
        running: [],
        paused: [],
        retrying: [{ identifier: 'X-1' }],
        maxConcurrentAgents: 5,
        agentMode: '',
      },
    });
    render(<StatusStrip />);
    expect(screen.getByText('1 retrying')).toBeInTheDocument();
  });

  it('renders capacity bar when maxConcurrentAgents > 0', () => {
    mockStore();
    render(<StatusStrip />);
    expect(screen.getByText('2/5')).toBeInTheDocument();
  });

  it('shows sub-agents badge when agentMode is subagents', () => {
    mockStore({
      snapshot: {
        running: [],
        paused: [],
        retrying: [],
        maxConcurrentAgents: 3,
        agentMode: 'subagents',
      },
    });
    render(<StatusStrip />);
    expect(screen.getByText('sub-agents')).toBeInTheDocument();
  });

  it('shows agent teams badge when agentMode is teams', () => {
    mockStore({
      snapshot: {
        running: [],
        paused: [],
        retrying: [],
        maxConcurrentAgents: 3,
        agentMode: 'teams',
      },
    });
    render(<StatusStrip />);
    expect(screen.getByText('agent teams')).toBeInTheDocument();
  });

  it('calls refreshSnapshot when + worker button is clicked', async () => {
    const store = mockStore();
    global.fetch = vi.fn().mockResolvedValue({ ok: true });
    render(<StatusStrip />);
    await userEvent.click(screen.getByTitle('Increase max workers'));
    expect(store.refreshSnapshot).toHaveBeenCalled();
  });
});
