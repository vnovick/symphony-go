import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import IssueDetailSlide from '../IssueDetailSlide';

vi.mock('../../../store/symphonyStore', () => ({ useSymphonyStore: vi.fn() }));

vi.mock('../../../queries/issues', () => ({
  useIssues: vi.fn(),
  useIssue: vi.fn(),
  useCancelIssue: vi.fn(),
  useTerminateIssue: vi.fn(),
  useResumeIssue: vi.fn(),
  useTriggerAIReview: vi.fn(),
  useSetIssueProfile: vi.fn(),
  useSetIssueBackend: vi.fn(),
  useProvideInput: vi.fn(),
  useDismissInput: vi.fn(),
  ISSUES_KEY: ['issues'],
}));

import { useSymphonyStore } from '../../../store/symphonyStore';
import * as issueQueries from '../../../queries/issues';

const mockStore = vi.mocked(useSymphonyStore);
const mockUseIssues = vi.mocked(issueQueries.useIssues);
const mockUseIssue = vi.mocked(issueQueries.useIssue);
const mockUseCancelIssue = vi.mocked(issueQueries.useCancelIssue);
const mockUseTerminateIssue = vi.mocked(issueQueries.useTerminateIssue);
const mockUseResumeIssue = vi.mocked(issueQueries.useResumeIssue);
const mockUseTriggerAIReview = vi.mocked(issueQueries.useTriggerAIReview);
const mockUseSetIssueProfile = vi.mocked(issueQueries.useSetIssueProfile);
const mockUseSetIssueBackend = vi.mocked(issueQueries.useSetIssueBackend);
const mockUseProvideInput = vi.mocked(issueQueries.useProvideInput);
const mockUseDismissInput = vi.mocked(issueQueries.useDismissInput);

const baseIssue = {
  identifier: 'ENG-10',
  title: 'Fix the bug',
  state: 'In Progress',
  orchestratorState: 'running' as const,
  description: 'A detailed description',
  comments: [] as { author: string; body: string; createdAt?: string }[],
  labels: [] as string[],
  priority: null as number | null,
  branchName: null as string | null,
  blockedBy: [] as string[],
  url: null as string | null,
  agentProfile: null as string | null,
};

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function setupDefaultMocks(
  selectedIdentifier: string | null,
  issueOverride?: Partial<typeof baseIssue>,
) {
  const setSelectedIdentifier = vi.fn();
  const issue = issueOverride ? { ...baseIssue, ...issueOverride } : baseIssue;

  mockStore.mockImplementation((selector: (s: any) => any) =>
    selector({
      selectedIdentifier,
      setSelectedIdentifier,
      snapshot: { availableProfiles: [] },
    }),
  );
  mockUseIssues.mockReturnValue({ data: [issue] } as any);
  mockUseIssue.mockReturnValue({ data: issue } as any);
  mockUseCancelIssue.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false } as any);
  mockUseTerminateIssue.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false } as any);
  mockUseResumeIssue.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false } as any);
  mockUseTriggerAIReview.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false } as any);
  mockUseSetIssueProfile.mockReturnValue({ mutate: vi.fn(), isPending: false } as any);
  mockUseSetIssueBackend.mockReturnValue({ mutate: vi.fn(), isPending: false } as any);
  mockUseProvideInput.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false } as any);
  mockUseDismissInput.mockReturnValue({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false } as any);

  return setSelectedIdentifier;
}

describe('IssueDetailSlide', () => {
  beforeEach(() => {
    setupDefaultMocks(null);
  });

  it('renders nothing when selectedIdentifier is null', () => {
    const { container } = render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when issue data is not available', () => {
    mockStore.mockImplementation((selector: (s: any) => any) =>
      selector({ selectedIdentifier: 'ENG-10', setSelectedIdentifier: vi.fn(), snapshot: { availableProfiles: [] } }),
    );
    mockUseIssues.mockReturnValue({ data: [] } as any);
    mockUseIssue.mockReturnValue({ data: undefined } as any);

    const { container } = render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(container.firstChild).toBeNull();
  });

  it('shows issue identifier when selected', () => {
    setupDefaultMocks('ENG-10');
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(screen.getByText('ENG-10')).toBeInTheDocument();
  });

  it('shows issue title', () => {
    setupDefaultMocks('ENG-10');
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(screen.getByText('Fix the bug')).toBeInTheDocument();
  });

  it('shows issue state badge', () => {
    setupDefaultMocks('ENG-10');
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(screen.getByText('In Progress')).toBeInTheDocument();
  });

  it('shows description content', async () => {
    setupDefaultMocks('ENG-10');
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(await screen.findByText('A detailed description')).toBeInTheDocument();
  });

  it('calls setSelectedIdentifier(null) when close button clicked', async () => {
    const setSelectedIdentifier = setupDefaultMocks('ENG-10');
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    const closeBtn = screen.getByRole('button', { name: /close/i });
    await userEvent.click(closeBtn);
    expect(setSelectedIdentifier).toHaveBeenCalledWith(null);
  });

  it('shows Pause Agent and Cancel Agent buttons when running', () => {
    setupDefaultMocks('ENG-10', { orchestratorState: 'running' });
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(screen.getByText(/Pause Agent/)).toBeInTheDocument();
    expect(screen.getByText(/Cancel Agent/)).toBeInTheDocument();
  });

  it('shows Resume Agent and Discard buttons when paused', () => {
    setupDefaultMocks('ENG-10', { orchestratorState: 'paused' });
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(screen.getByText(/Resume Agent/)).toBeInTheDocument();
    expect(screen.getByText(/Discard/)).toBeInTheDocument();
  });

  it('shows comments when present', async () => {
    setupDefaultMocks('ENG-10', {
      comments: [{ author: 'alice', body: 'Looks good to me', createdAt: '2024-01-01T00:00:00Z' }],
    });
    render(<IssueDetailSlide />, { wrapper: makeWrapper() });
    expect(await screen.findByText('Looks good to me')).toBeInTheDocument();
    expect(screen.getByText('alice')).toBeInTheDocument();
  });
});
