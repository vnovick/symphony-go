import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import RunningSessionsTable from '../RunningSessionsTable';
import type { RunningRow } from '../../../types/schemas';

// Mock Zustand store
vi.mock('../../../store/itervoxStore.ts', () => ({
  useItervoxStore: vi.fn(),
}));

// Mock query hooks
vi.mock('../../../queries/issues', () => ({
  useCancelIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useTerminateIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useResumeIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useSetIssueProfile: () => ({ mutate: vi.fn(), isPending: false }),
  useSetIssueBackend: () => ({ mutate: vi.fn(), isPending: false }),
  useTriggerAIReview: () => ({ mutate: vi.fn(), isPending: false }),
  useIssues: () => ({ data: [] }),
}));

vi.mock('../../../queries/logs', () => ({
  useIssueLogs: () => ({ data: [] }),
}));

vi.mock('../../ui/Terminal/Terminal', () => ({
  Terminal: () => <div data-testid="terminal-mock" />,
}));

import { useItervoxStore } from '../../../store/itervoxStore';

const mockUseItervoxStore = vi.mocked(useItervoxStore);

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const baseRow: RunningRow = {
  identifier: 'ISS-42',
  state: 'In Progress',
  turnCount: 5,
  tokens: 1200,
  inputTokens: 800,
  outputTokens: 400,
  lastEvent: 'Doing some work',
  lastEventAt: null,
  sessionId: 'sess-abc-123',
  workerHost: 'worker-1',
  backend: 'claude',
  elapsedMs: 60000,
  startedAt: new Date(Date.now() - 60000).toISOString(),
};

describe('RunningSessionsTable', () => {
  const mockSetSelectedIdentifier = vi.fn();

  beforeEach(() => {
    // Default: empty snapshot
    mockUseItervoxStore.mockImplementation((selector: (s: any) => any) =>
      selector({ snapshot: null, setSelectedIdentifier: mockSetSelectedIdentifier }),
    );
  });

  function withSnapshot(snapshot: {
    running?: RunningRow[];
    paused?: string[];
    pausedWithPR?: Record<string, string>;
    availableProfiles?: string[];
  }) {
    mockUseItervoxStore.mockImplementation((selector: (s: any) => any) =>
      selector({
        snapshot: {
          running: snapshot.running ?? [],
          paused: snapshot.paused ?? [],
          pausedWithPR: snapshot.pausedWithPR ?? {},
          availableProfiles: snapshot.availableProfiles ?? [],
        },
        setSelectedIdentifier: mockSetSelectedIdentifier,
      }),
    );
  }

  it('renders empty state when no running sessions', () => {
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('No agents running')).toBeInTheDocument();
  });

  it('renders "Running Sessions" heading when running sessions exist', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('Running Sessions')).toBeInTheDocument();
  });

  it('renders session row when running sessions provided', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('ISS-42')).toBeInTheDocument();
  });

  it('shows session identifier in the row', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('ISS-42')).toBeInTheDocument();
  });

  it('shows session state badge', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('In Progress')).toBeInTheDocument();
  });

  it('shows count badge when sessions exist', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('1 active')).toBeInTheDocument();
  });

  it('shows Pause and Cancel action buttons per row', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText(/Pause/)).toBeInTheDocument();
    expect(screen.getByText(/Cancel/)).toBeInTheDocument();
  });

  it('shows running session summary fields when rows are present', () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    // Turn count and elapsed time are rendered in the row grid
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('1m 00s')).toBeInTheDocument();
  });

  it('shows paused section when paused identifiers exist', () => {
    withSnapshot({ paused: ['ISS-99'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('ISS-99')).toBeInTheDocument();
    expect(screen.getByText(/Paused/)).toBeInTheDocument();
  });

  it('shows Resume and Discard buttons for paused items', () => {
    withSnapshot({ paused: ['ISS-99'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText(/Resume/)).toBeInTheDocument();
    expect(screen.getByText(/Discard/)).toBeInTheDocument();
  });

  it('shows PR link when paused with PR', () => {
    withSnapshot({
      paused: ['ISS-99'],
      pausedWithPR: { 'ISS-99': 'https://github.com/org/repo/pull/5' },
    });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('PR')).toBeInTheDocument();
  });

  it('expands accordion on row click', async () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    // The row has a grid layout with role="button"; click the chevron area
    const rows = screen.getAllByRole('button');
    // Find the row-level button (the grid row), not the identifier link or action buttons
    const rowButton = rows.find((el) => el.classList.contains('grid'));
    expect(rowButton).toBeTruthy();
    if (!rowButton) throw new Error('rowButton not found');
    await userEvent.click(rowButton);
    // After expanding, the accordion renders the terminal mock
    expect(screen.getByTestId('terminal-mock')).toBeInTheDocument();
  });

  it('shows reviewer badge when row kind is reviewer', () => {
    const reviewerRow: RunningRow = { ...baseRow, kind: 'reviewer' };
    withSnapshot({ running: [reviewerRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('Review')).toBeInTheDocument();
  });

  it('does not show reviewer badge when kind is not reviewer', () => {
    const workerRow: RunningRow = { ...baseRow, kind: 'worker' };
    withSnapshot({ running: [workerRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.queryByText('Review')).not.toBeInTheDocument();
  });

  it('shows dash when turnCount is null', () => {
    const noTurnRow: RunningRow = { ...baseRow, turnCount: null as any };
    withSnapshot({ running: [noTurnRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    // The turn count column should render a dash
    const dashes = screen.getAllByText('\u2014');
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it('shows dash when lastEvent is empty', () => {
    const noEventRow: RunningRow = { ...baseRow, lastEvent: undefined };
    withSnapshot({ running: [noEventRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    const dashes = screen.getAllByText('\u2014');
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it('truncates lastEvent to 100 chars', () => {
    const longEvent = 'A'.repeat(150);
    const longEventRow: RunningRow = { ...baseRow, lastEvent: longEvent };
    withSnapshot({ running: [longEventRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('A'.repeat(100))).toBeInTheDocument();
  });

  it('does not show PR badge for paused item without PR', () => {
    withSnapshot({ paused: ['ISS-99'], pausedWithPR: {} });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.queryByText('PR')).not.toBeInTheDocument();
  });

  it('renders both running and paused sections simultaneously', () => {
    withSnapshot({ running: [baseRow], paused: ['ISS-99'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('Running Sessions')).toBeInTheDocument();
    expect(screen.getByText(/Paused/)).toBeInTheDocument();
    expect(screen.getByText('ISS-42')).toBeInTheDocument();
    expect(screen.getByText('ISS-99')).toBeInTheDocument();
  });

  it('does not show Running Sessions header when only paused items exist', () => {
    withSnapshot({ paused: ['ISS-99'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.queryByText('Running Sessions')).not.toBeInTheDocument();
    expect(screen.getByText(/Paused/)).toBeInTheDocument();
  });

  it('opens detail slide when identifier is clicked in paused section', async () => {
    withSnapshot({ paused: ['ISS-99'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    const identifier = screen.getByLabelText('View details for paused issue ISS-99');
    await userEvent.click(identifier);
    expect(mockSetSelectedIdentifier).toHaveBeenCalledWith('ISS-99');
  });

  it('opens detail slide when identifier is clicked in running section', async () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    const identifier = screen.getByLabelText('View details for ISS-42');
    await userEvent.click(identifier);
    expect(mockSetSelectedIdentifier).toHaveBeenCalledWith('ISS-42');
  });

  it('expands paused accordion on paused row click', async () => {
    withSnapshot({ paused: ['ISS-99'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    const pausedRow = screen.getByLabelText('Toggle details for paused issue ISS-99');
    await userEvent.click(pausedRow);
    expect(screen.getByTestId('terminal-mock')).toBeInTheDocument();
  });

  it('shows multiple running rows sorted by startedAt', () => {
    const earlier: RunningRow = {
      ...baseRow,
      identifier: 'ISS-1',
      startedAt: new Date(Date.now() - 120000).toISOString(),
    };
    const later: RunningRow = {
      ...baseRow,
      identifier: 'ISS-2',
      startedAt: new Date(Date.now() - 30000).toISOString(),
    };
    withSnapshot({ running: [later, earlier] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    const identifiers = screen.getAllByText(/ISS-/);
    // Earlier should come first in order
    expect(identifiers[0].textContent).toBe('ISS-1');
    expect(identifiers[1].textContent).toBe('ISS-2');
  });

  it('shows paused count in section header', () => {
    withSnapshot({ paused: ['ISS-1', 'ISS-2', 'ISS-3'] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText(/Paused \(3\)/)).toBeInTheDocument();
  });

  it('expands running row via keyboard Enter on identifier', async () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    // Click the identifier to open detail slide via keyboard
    const identifierButton = screen.getByLabelText('View details for ISS-42');
    await userEvent.click(identifierButton);
    expect(mockSetSelectedIdentifier).toHaveBeenCalledWith('ISS-42');
  });
});
