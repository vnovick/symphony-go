import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PausedIssuePanel } from '../PausedIssuePanel';
import type { TrackerIssue } from '../../../types/schemas';

const wrapper = ({ children }: { children: React.ReactNode }) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
};

const pausedIssue: TrackerIssue = {
  identifier: 'PROJ-42',
  title: 'Fix auth timeout',
  state: 'In Progress',
  orchestratorState: 'paused',
};

describe('PausedIssuePanel', () => {
  it('renders backend toggle with Claude and Codex options', () => {
    render(
      <PausedIssuePanel
        isOpen={true}
        issue={pausedIssue}
        onClose={vi.fn()}
        availableProfiles={['reviewer', 'architect']}
        defaultBackend="claude"
      />,
      { wrapper },
    );
    expect(screen.getByText('Claude')).toBeInTheDocument();
    expect(screen.getByText('Codex')).toBeInTheDocument();
  });

  it('renders profile chips when profiles available', () => {
    render(
      <PausedIssuePanel
        isOpen={true}
        issue={pausedIssue}
        onClose={vi.fn()}
        availableProfiles={['reviewer', 'architect']}
        defaultBackend="claude"
      />,
      { wrapper },
    );
    expect(screen.getByText('reviewer')).toBeInTheDocument();
    expect(screen.getByText('architect')).toBeInTheDocument();
    expect(screen.getByText('default')).toBeInTheDocument();
  });

  it('shows resume and terminate buttons', () => {
    render(
      <PausedIssuePanel
        isOpen={true}
        issue={pausedIssue}
        onClose={vi.fn()}
        availableProfiles={[]}
        defaultBackend="claude"
      />,
      { wrapper },
    );
    expect(screen.getByText(/Resume/)).toBeInTheDocument();
    expect(screen.getByText(/Terminate/)).toBeInTheDocument();
  });
});
