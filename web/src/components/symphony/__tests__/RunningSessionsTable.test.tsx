import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import RunningSessionsTable from '../RunningSessionsTable';
import type { RunningRow } from '../../../types/symphony';

// Mock Zustand store
vi.mock('../../../store/symphonyStore', () => ({
  useSymphonyStore: vi.fn(),
}));

// Mock query hooks
vi.mock('../../../queries/issues', () => ({
  useCancelIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useTerminateIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useResumeIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useReanalyzeIssue: () => ({ mutate: vi.fn(), isPending: false }),
  useSetIssueProfile: () => ({ mutate: vi.fn(), isPending: false }),
  useTriggerAIReview: () => ({ mutate: vi.fn(), isPending: false }),
  useIssues: () => ({ data: [] }),
}));

vi.mock('../../../queries/logs', () => ({
  useIssueLogs: () => ({ data: [] }),
}));

import { useSymphonyStore } from '../../../store/symphonyStore';

const mockUseSymphonyStore = vi.mocked(useSymphonyStore);

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
    mockUseSymphonyStore.mockImplementation((selector: (s: any) => any) =>
      selector({ snapshot: null, setSelectedIdentifier: mockSetSelectedIdentifier }),
    );
  });

  function withSnapshot(snapshot: {
    running?: RunningRow[];
    paused?: string[];
    pausedWithPR?: Record<string, string>;
  }) {
    mockUseSymphonyStore.mockImplementation((selector: (s: any) => any) =>
      selector({
        snapshot: {
          running: snapshot.running ?? [],
          paused: snapshot.paused ?? [],
          pausedWithPR: snapshot.pausedWithPR ?? {},
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
    expect(screen.getByText('claude')).toBeInTheDocument();
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

  it('shows Open PR link and Re-analyze button when paused with PR', () => {
    withSnapshot({
      paused: ['ISS-99'],
      pausedWithPR: { 'ISS-99': 'https://github.com/org/repo/pull/5' },
    });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    expect(screen.getByText('Open PR')).toBeInTheDocument();
    expect(screen.getByText(/Re-analyze/)).toBeInTheDocument();
  });

  it('expands accordion on row click', async () => {
    withSnapshot({ running: [baseRow] });
    render(<RunningSessionsTable />, { wrapper: makeWrapper() });
    const row = screen.getByText('ISS-42').closest('[class*="cursor-pointer"]') as HTMLElement;
    await userEvent.click(row);
    // accordion header "Logs" should appear
    expect(screen.getByText('Logs')).toBeInTheDocument();
  });
});
