import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MetricsStrip } from '../MetricsStrip';

// Mock symphonyStore
vi.mock('../../../store/symphonyStore', () => ({
  useSymphonyStore: vi.fn(),
}));

import { useSymphonyStore } from '../../../store/symphonyStore';

const mockStore = useSymphonyStore as ReturnType<typeof vi.fn>;

function makeSnapshot(overrides: Record<string, unknown> = {}) {
  return {
    counts: { running: 2, retrying: 1, paused: 0 },
    running: [{}, {}],
    retrying: [{}],
    paused: [],
    maxConcurrentAgents: 5,
    rateLimits: null,
    ...overrides,
  };
}

beforeEach(() => {
  mockStore.mockImplementation((selector: (s: unknown) => unknown) =>
    selector({ snapshot: makeSnapshot() }),
  );
});

describe('MetricsStrip', () => {
  it('renders Running label', () => {
    render(<MetricsStrip />);
    expect(screen.getByText('Running')).toBeInTheDocument();
  });

  it('renders Backoff label', () => {
    render(<MetricsStrip />);
    expect(screen.getByText('Backoff')).toBeInTheDocument();
  });

  it('renders Paused label', () => {
    render(<MetricsStrip />);
    expect(screen.getByText('Paused')).toBeInTheDocument();
  });

  it('renders Capacity label', () => {
    render(<MetricsStrip />);
    expect(screen.getByText('Capacity')).toBeInTheDocument();
  });

  it('shows running count from snapshot', () => {
    render(<MetricsStrip />);
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('shows capacity as fraction', () => {
    render(<MetricsStrip />);
    expect(screen.getByText('2/5')).toBeInTheDocument();
  });

  it('renders without crashing when snapshot is null', () => {
    mockStore.mockImplementation((selector: (s: unknown) => unknown) =>
      selector({ snapshot: null }),
    );
    const { container } = render(<MetricsStrip />);
    expect(container.firstChild).toBeInTheDocument();
  });
});
