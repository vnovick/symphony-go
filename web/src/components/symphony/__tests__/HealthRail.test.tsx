import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { HealthRail } from '../HealthRail';

vi.mock('../../../store/symphonyStore', () => ({ useSymphonyStore: vi.fn() }));

import { useSymphonyStore } from '../../../store/symphonyStore';

const mockStore = vi.mocked(useSymphonyStore);

const baseSnapshot = {
  counts: { running: 2, retrying: 1, paused: 0 },
  maxConcurrentAgents: 5,
  rateLimits: null,
};

function withSnapshot(override?: Partial<typeof baseSnapshot>) {
  mockStore.mockImplementation((selector: (s: any) => any) =>
    selector({ snapshot: { ...baseSnapshot, ...override } }),
  );
}

describe('HealthRail', () => {
  beforeEach(() => {
    mockStore.mockImplementation((selector: (s: any) => any) =>
      selector({ snapshot: null }),
    );
  });

  it('renders nothing when snapshot is null', () => {
    const { container } = render(<HealthRail />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the rail when snapshot is present', () => {
    withSnapshot();
    render(<HealthRail />);
    expect(screen.getByTestId('health-rail')).toBeInTheDocument();
  });

  it('shows worker saturation as running/max', () => {
    withSnapshot();
    render(<HealthRail />);
    expect(screen.getByText('2 / 5')).toBeInTheDocument();
  });

  it('shows retrying count as retry pressure', () => {
    withSnapshot();
    render(<HealthRail />);
    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('shows blocked badge when paused issues exist', () => {
    withSnapshot({ counts: { running: 1, retrying: 0, paused: 3 } });
    render(<HealthRail />);
    expect(screen.getByText(/3 blocked/i)).toBeInTheDocument();
  });

  it('does not show blocked badge when paused is zero', () => {
    withSnapshot({ counts: { running: 2, retrying: 0, paused: 0 } });
    render(<HealthRail />);
    expect(screen.queryByText(/blocked/i)).not.toBeInTheDocument();
  });

  it('shows capacity label', () => {
    withSnapshot();
    render(<HealthRail />);
    expect(screen.getByText(/capacity/i)).toBeInTheDocument();
  });
});
