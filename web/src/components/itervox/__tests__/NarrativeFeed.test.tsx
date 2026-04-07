import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { NarrativeFeed } from '../NarrativeFeed';

vi.mock('../../../store/itervoxStore', () => ({ useItervoxStore: vi.fn() }));

import { useItervoxStore } from '../../../store/itervoxStore';

const mockStore = vi.mocked(useItervoxStore);

describe('NarrativeFeed', () => {
  beforeEach(() => {
    mockStore.mockImplementation((selector: (s: any) => any) => selector({ logs: [] }));
  });

  it('renders the feed container', () => {
    render(<NarrativeFeed />);
    expect(screen.getByTestId('narrative-feed')).toBeInTheDocument();
  });

  it('shows empty state when no log lines', () => {
    render(<NarrativeFeed />);
    expect(screen.getByText(/no events/i)).toBeInTheDocument();
  });

  it('renders log messages from the store', () => {
    mockStore.mockImplementation((selector: (s: any) => any) =>
      selector({ logs: ['agent started', 'running tool'] }),
    );
    render(<NarrativeFeed />);
    expect(screen.getByText('agent started')).toBeInTheDocument();
    expect(screen.getByText('running tool')).toBeInTheDocument();
  });

  it('shows at most 20 entries', () => {
    const lines = Array.from({ length: 25 }, (_, i) => `event ${String(i)}`);
    mockStore.mockImplementation((selector: (s: any) => any) => selector({ logs: lines }));
    render(<NarrativeFeed />);
    // Shows last 20 events (indices 5-24)
    expect(screen.queryByText('event 0')).not.toBeInTheDocument();
    expect(screen.getByText('event 24')).toBeInTheDocument();
  });

  it('shows the section heading', () => {
    render(<NarrativeFeed />);
    expect(screen.getByText(/recent events/i)).toBeInTheDocument();
  });
});
