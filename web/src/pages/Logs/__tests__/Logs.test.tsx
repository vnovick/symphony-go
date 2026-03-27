import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Logs from '../index';
import type { IssueLogEntry } from '../../../types/schemas';

// ─── Mocks ────────────────────────────────────────────────────────────────────

vi.mock('zustand/react/shallow', () => ({
  useShallow: (fn: unknown) => fn,
}));

vi.mock('../../../store/symphonyStore', () => ({
  useSymphonyStore: vi.fn(),
}));

vi.mock('../../../queries/issues', () => ({
  useIssues: vi.fn(),
  useClearIssueLogs: () => ({ mutate: vi.fn(), isPending: false }),
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

import { useSymphonyStore } from '../../../store/symphonyStore';
import { useIssues } from '../../../queries/issues';
import { useIssueLogs, useLogIdentifiers } from '../../../queries/logs';

const mockUseSymphonyStore = vi.mocked(useSymphonyStore);
const mockUseIssues = vi.mocked(useIssues);
const mockUseIssueLogs = vi.mocked(useIssueLogs);
const mockUseLogIdentifiers = vi.mocked(useLogIdentifiers);

function makeEntry(event: string, message: string): IssueLogEntry {
  return { event, message, level: 'INFO', tool: '', time: '' } as unknown as IssueLogEntry;
}

function makeIssue(identifier: string) {
  return {
    identifier, title: `Title ${identifier}`, state: 'In Progress',
    description: '', url: '', orchestratorState: 'idle',
    turnCount: 0, tokens: 0, elapsedMs: 0, lastMessage: '', error: '',
  };
}

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return React.createElement(QueryClientProvider, { client: qc }, children);
}

function setupStoreMock(activeIssueId: string | null = null) {
  mockUseSymphonyStore.mockImplementation((sel: (s: any) => any) =>
    sel({
      snapshot: { running: [], retrying: [] },
      activeIssueId,
      setActiveIssueId: vi.fn(),
    }),
  );
}

beforeEach(() => {
  setupStoreMock(null);
  mockUseIssues.mockReturnValue({ data: [] } as ReturnType<typeof useIssues>);
  mockUseIssueLogs.mockReturnValue({ data: [], isLoading: false } as ReturnType<typeof useIssueLogs>);
  mockUseLogIdentifiers.mockReturnValue([]);
});

describe('Logs page', () => {
  it('shows all filter chips by default', () => {
    setupStoreMock('ABC-1');
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
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
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<typeof useIssueLogs>);
    render(<Logs />, { wrapper });
    await waitFor(() => { expect(screen.getAllByTestId('terminal-entry')).toHaveLength(3); });
    await user.click(screen.getByTestId('chip-action'));
    await waitFor(() => { expect(screen.getAllByTestId('terminal-entry')).toHaveLength(2); });
  });

  it('shows entries again when a chip is re-activated', async () => {
    setupStoreMock('ABC-1');
    const user = userEvent.setup();
    const entries: IssueLogEntry[] = [makeEntry('text', 'Hello'), makeEntry('action', 'Tool call')];
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<typeof useIssueLogs>);
    render(<Logs />, { wrapper });
    await waitFor(() => { expect(screen.getAllByTestId('terminal-entry')).toHaveLength(2); });
    await user.click(screen.getByTestId('chip-action'));
    await user.click(screen.getByTestId('chip-action'));
    await waitFor(() => { expect(screen.getAllByTestId('terminal-entry')).toHaveLength(2); });
  });

  it('passes correct entry messages to Terminal', async () => {
    setupStoreMock('ABC-1');
    const entries: IssueLogEntry[] = [makeEntry('text', 'first line'), makeEntry('text', 'second line')];
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<typeof useIssueLogs>);
    render(<Logs />, { wrapper });
    await waitFor(() => { expect(screen.getByText('first line')).toBeInTheDocument(); });
    expect(screen.getByText('second line')).toBeInTheDocument();
  });

  it('shows context strip when an issue is selected', () => {
    setupStoreMock('ABC-1');
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    render(<Logs />, { wrapper });
    expect(screen.getByTestId('logs-context-strip')).toBeInTheDocument();
  });

  it('passes tool name as prefix in entry message', async () => {
    setupStoreMock('ABC-1');
    const entries: IssueLogEntry[] = [
      { event: 'action', message: 'reading file', level: 'INFO', tool: 'Read', time: '' } as unknown as IssueLogEntry,
    ];
    mockUseIssues.mockReturnValue({ data: [makeIssue('ABC-1')] } as ReturnType<typeof useIssues>);
    mockUseLogIdentifiers.mockReturnValue(['ABC-1']);
    mockUseIssueLogs.mockReturnValue({ data: entries, isLoading: false } as ReturnType<typeof useIssueLogs>);
    render(<Logs />, { wrapper });
    await waitFor(() => { expect(screen.getByText(/Read.*reading file/)).toBeInTheDocument(); });
  });
});
