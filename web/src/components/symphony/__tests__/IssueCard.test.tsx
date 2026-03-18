import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import IssueCard from '../IssueCard';
import type { TrackerIssue } from '../../../types/symphony';

const baseIssue: TrackerIssue = {
  identifier: 'ABC-1',
  title: 'Fix the bug',
  state: 'In Progress',
  description: '',
  url: 'https://example.com/ABC-1',
  orchestratorState: 'running',
  turnCount: 3,
  tokens: 1000,
  elapsedMs: 90000,
  lastMessage: '',
  error: '',
};

describe('IssueCard', () => {
  it('renders identifier and title', () => {
    render(<IssueCard issue={baseIssue} onSelect={vi.fn()} />);
    expect(screen.getByText('ABC-1')).toBeInTheDocument();
    expect(screen.getByText('Fix the bug')).toBeInTheDocument();
  });

  it('renders elapsed time when elapsedMs > 0', () => {
    render(<IssueCard issue={baseIssue} onSelect={vi.fn()} />);
    expect(screen.getByText(/1m 30s/)).toBeInTheDocument();
  });

  it('does not render elapsed when elapsedMs is 0', () => {
    render(<IssueCard issue={{ ...baseIssue, elapsedMs: 0 }} onSelect={vi.fn()} />);
    expect(screen.queryByText(/⏱/)).not.toBeInTheDocument();
  });

  it('calls onSelect with identifier when clicked', async () => {
    const onSelect = vi.fn();
    render(<IssueCard issue={baseIssue} onSelect={onSelect} />);
    await userEvent.click(screen.getByText('Fix the bug'));
    expect(onSelect).toHaveBeenCalledWith('ABC-1');
  });

  it('renders URL as a link', () => {
    render(<IssueCard issue={baseIssue} onSelect={vi.fn()} />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', 'https://example.com/ABC-1');
  });

  it('renders identifier as plain text when no url', () => {
    render(<IssueCard issue={{ ...baseIssue, url: '' }} onSelect={vi.fn()} />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.getByText('ABC-1')).toBeInTheDocument();
  });

  it('applies dragging styles when isDragging is true', () => {
    const { container } = render(<IssueCard issue={baseIssue} onSelect={vi.fn()} isDragging />);
    expect(container.firstChild).toHaveClass('rotate-1');
  });
});
