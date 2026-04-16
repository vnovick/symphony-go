import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Logs from '../index';
import type { IssueLogEntry, TrackerIssue } from '../../../types/schemas';

// ─── Mocks ────────────────────────────────────────────────────────────────────

vi.mock('zustand/react/shallow', () => ({
  useShallow: (fn: unknown) => fn,
}));

vi.mock('../../../store/itervoxStore.ts', () => ({
  useItervoxStore: vi.fn(),
}));

vi.mock('../../../queries/issues', () => ({
  useClearIssueLogs: () => ({ mutate: vi.fn(), isPending: false }),
  useIssues: vi.fn(),
}));

vi.mock('../../../queries/logs', () => ({
  useIssueLogs: vi.fn(),
  useLogIdentifiers: vi.fn(),
}));

vi.mock('../../../components/ui/Terminal/Terminal', () => ({
  Terminal: ({ entries }: { entries: Array<{ message: string }> }) => (
    <div data-testid="terminal">
      {entries.map((e, i) => (
        <div key={i} data-testid="terminal-entry">
          {e.message}
        </div>
      ))}
    </div>
  ),
}));

import { useItervoxStore } from '../../../store/itervoxStore';
import { useIssues } from '../../../queries/issues';
import { useIssueLogs, useLogIdentifiers } from '../../../queries/logs';

const mockuseItervoxStore = vi.mocked(useItervoxStore);
const mockUseIssues = vi.mocked(useIssues);
const mockUseIssueLogs = vi.mocked(useIssueLogs);
const mockUseLogIdentifiers = vi.mocked(useLogIdentifiers);

function makeEntry(event: string, message: string): IssueLogEntry {
  return { event, message, level: 'INFO', tool: '', time: '' } as unknown as IssueLogEntry;
}

function makeIssue(identifier: string, overrides: Partial<TrackerIssue> = {}): TrackerIssue {
  return {
    identifier,
    title: `${identifier} title`,
    state: 'In Progress',
    orchestratorState: 'idle',
    branchName: null,
    ...overrides,
  } as TrackerIssue;
}

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return React.createElement(QueryClientProvider, { client: qc }, children);
}

function setupStoreMock(
  activeIssueId: string | null = null,
  snapshotOverride?: Partial<{
    inputRequired: Array<{
      identifier: string;
      sessionId?: string;
      state?: 'input_required' | 'pending_input_resume';
      context?: string;
      queuedAt?: string;
    }>;
    paused: string[];
    running: Array<{ identifier: string; workerHost?: string; sessionId?: string }>;
    retrying: Array<{ identifier: string }>;
  }>,
) {
  mockuseItervoxStore.mockImplementation((sel: (s: any) => any) =>
    sel({
      snapshot: {
        inputRequired: snapshotOverride?.inputRequired ?? [],
        paused: snapshotOverride?.paused ?? [],
        running: snapshotOverride?.running ?? [],
        retrying: snapshotOverride?.retrying ?? [],
      },
      activeIssueId,
      setActiveIssueId: vi.fn(),
    }),
  );
}

beforeEach(() => {
  setupStoreMock(null);
  mockUseIssues.mockReturnValue({ data: [] } as ReturnType<typeof useIssues>);
  mockUseIssueLogs.mockReturnValue({ data: [], isLoading: false } as ReturnType<
    typeof useIssueLogs
  >);
  mockUseLogIdentifiers.mockReturnValue([]);
});

describe('Logs page', () => {
  it('shows all filter chips by default', () => {
    setupStoreMock('ABC-1');
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    render(<Logs />, { wrapper });
    expect(screen.getByTestId('chip-text')).toBeInTheDocument();
    expect(screen.getByTestId('chip-action')).toBeInTheDocument();
    expect(screen.getByTestId('chip-subagent')).toBeInTheDocument();
    expect(screen.getByTestId('chip-warn')).toBeInTheDocument();
    expect(screen.getByTestId('chip-error')).toBeInTheDocument();
  });

  it('hides entries of a deactivated chip type', async () => {
    setupStoreMock('ABC-1');
    const user = userEvent.setup();
    const entries: IssueLogEntry[] = [
      makeEntry('text', 'Hello world'),
      makeEntry('action', 'Tool call'),
      makeEntry('subagent', 'Spawning subagent'),
    ];
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<
      typeof useIssueLogs
    >);
    render(<Logs />, { wrapper });
    await waitFor(() => {
      expect(screen.getAllByTestId('terminal-entry')).toHaveLength(3);
    });
    await user.click(screen.getByTestId('chip-action'));
    await waitFor(() => {
      expect(screen.getAllByTestId('terminal-entry')).toHaveLength(2);
    });
  });

  it('shows entries again when a chip is re-activated', async () => {
    setupStoreMock('ABC-1');
    const user = userEvent.setup();
    const entries: IssueLogEntry[] = [makeEntry('text', 'Hello'), makeEntry('action', 'Tool call')];
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<
      typeof useIssueLogs
    >);
    render(<Logs />, { wrapper });
    await waitFor(() => {
      expect(screen.getAllByTestId('terminal-entry')).toHaveLength(2);
    });
    await user.click(screen.getByTestId('chip-action'));
    await user.click(screen.getByTestId('chip-action'));
    await waitFor(() => {
      expect(screen.getAllByTestId('terminal-entry')).toHaveLength(2);
    });
  });

  it('passes correct entry messages to Terminal', async () => {
    setupStoreMock('ABC-1');
    const entries: IssueLogEntry[] = [
      makeEntry('text', 'first line'),
      makeEntry('text', 'second line'),
    ];
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<
      typeof useIssueLogs
    >);
    render(<Logs />, { wrapper });
    await waitFor(() => {
      expect(screen.getByText('first line')).toBeInTheDocument();
    });
    expect(screen.getByText('second line')).toBeInTheDocument();
  });

  it('shows context strip when an issue is selected', () => {
    setupStoreMock('ABC-1');
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    render(<Logs />, { wrapper });
    expect(screen.getByTestId('logs-context-strip')).toBeInTheDocument();
  });

  it('uses snapshot input-required state for the selected issue', () => {
    setupStoreMock('ABC-1', {
      inputRequired: [{ identifier: 'ABC-1', state: 'input_required', context: 'Need approval' }],
    });
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    render(<Logs />, { wrapper });
    expect(screen.getAllByText(/input required/i)).toHaveLength(2);
  });

  it('shows pending resume state from snapshot context for the selected issue', () => {
    setupStoreMock('ABC-1', {
      inputRequired: [
        {
          identifier: 'ABC-1',
          state: 'pending_input_resume',
          context: 'Reply received, waiting to resume.\n\nOriginal request:\nNeed approval',
        },
      ],
    });
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    render(<Logs />, { wrapper });
    expect(screen.getAllByText(/reply received/i)).toHaveLength(2);
  });

  it('passes tool name as prefix in entry message', async () => {
    setupStoreMock('ABC-1');
    const entries: IssueLogEntry[] = [
      {
        event: 'action',
        message: 'reading file',
        level: 'INFO',
        tool: 'Read',
        time: '',
      } as unknown as IssueLogEntry,
    ];
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<
      typeof useIssueLogs
    >);
    render(<Logs />, { wrapper });
    await waitFor(() => {
      expect(screen.getByText(/Read.*reading file/)).toBeInTheDocument();
    });
  });

  it('shows live issues without logs in the sidebar', () => {
    setupStoreMock(null, { running: [{ identifier: 'ABC-1' }] });
    mockUseIssues.mockReturnValue({
      data: [makeIssue('ABC-1', { orchestratorState: 'running' })],
    } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue([]);
    render(<Logs />, { wrapper });
    expect(screen.getByText('ABC-1')).toBeInTheDocument();
    expect(screen.getByText('1 active · 1 total')).toBeInTheDocument();
  });

  it('does not show idle issues that have no logs', () => {
    setupStoreMock(null);
    mockUseIssues.mockReturnValue({
      data: [makeIssue('ABC-1', { orchestratorState: 'idle' })],
    } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue([]);
    render(<Logs />, { wrapper });
    expect(screen.queryByText('ABC-1')).not.toBeInTheDocument();
    expect(screen.getByText('0 active · 0 total')).toBeInTheDocument();
  });

  it('restores branch and profile context in the header strip', () => {
    setupStoreMock('ABC-1', {
      running: [{ identifier: 'ABC-1', workerHost: 'ssh-1', sessionId: '12345678abcd' }],
    });
    mockUseIssues.mockReturnValue({
      data: [
        makeIssue('ABC-1', {
          branchName: 'feature/abc-1',
          agentProfile: 'reviewer',
        }),
      ],
    } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    render(<Logs />, { wrapper });
    expect(screen.getByText('feature/abc-1')).toBeInTheDocument();
    expect(screen.getByText('reviewer')).toBeInTheDocument();
    expect(screen.getByText('ssh-1')).toBeInTheDocument();
  });
});
