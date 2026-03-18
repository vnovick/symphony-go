import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import BoardColumn from '../BoardColumn';
import type { TrackerIssue } from '../../../types/symphony';

vi.mock('@dnd-kit/core', () => ({
  useDraggable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    transform: null,
    isDragging: false,
  }),
  useDroppable: () => ({ setNodeRef: vi.fn(), isOver: false }),
  DndContext: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  closestCenter: vi.fn(),
  // eslint-disable-next-line @typescript-eslint/no-extraneous-class
  PointerSensor: class {},
  useSensor: vi.fn(),
  useSensors: vi.fn(() => []),
}));

vi.mock('@dnd-kit/utilities', () => ({
  CSS: {
    Translate: { toString: vi.fn(() => '') },
  },
}));

// IssueCard renders the issue; mock it to keep tests focused on BoardColumn
vi.mock('../IssueCard', () => ({
  default: ({ issue, onSelect }: { issue: TrackerIssue; onSelect: (id: string) => void }) => (
    <div
      data-testid="issue-card"
      onClick={() => {
        onSelect(issue.identifier);
      }}
    >
      {issue.identifier}
    </div>
  ),
}));

const makeIssue = (id: string): TrackerIssue => ({
  identifier: id,
  title: `Title ${id}`,
  state: 'In Progress',
  description: '',
  url: '',
  orchestratorState: 'idle',
  turnCount: 0,
  tokens: 0,
  elapsedMs: 0,
  lastMessage: '',
  error: '',
});

describe('BoardColumn', () => {
  it('renders the column state label', () => {
    render(<BoardColumn state="In Progress" issues={[]} isOver={false} onSelect={vi.fn()} />);
    expect(screen.getByText('In Progress')).toBeInTheDocument();
  });

  it('shows the issue count badge', () => {
    const issues = [makeIssue('ABC-1'), makeIssue('ABC-2')];
    render(<BoardColumn state="Todo" issues={issues} isOver={false} onSelect={vi.fn()} />);
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('shows 0 in count badge when no issues', () => {
    render(<BoardColumn state="Done" issues={[]} isOver={false} onSelect={vi.fn()} />);
    expect(screen.getByText('0')).toBeInTheDocument();
  });

  it('renders a card for each issue', () => {
    const issues = [makeIssue('XYZ-1'), makeIssue('XYZ-2'), makeIssue('XYZ-3')];
    render(<BoardColumn state="Backlog" issues={issues} isOver={false} onSelect={vi.fn()} />);
    expect(screen.getAllByTestId('issue-card')).toHaveLength(3);
  });

  it('applies highlighted styles when isOver is true', () => {
    const { container } = render(
      <BoardColumn state="In Progress" issues={[]} isOver={true} onSelect={vi.fn()} />,
    );
    expect(container.firstChild).toHaveClass('border-brand-300');
  });

  it('applies default styles when isOver is false', () => {
    const { container } = render(
      <BoardColumn state="In Progress" issues={[]} isOver={false} onSelect={vi.fn()} />,
    );
    expect(container.firstChild).toHaveClass('border-gray-200');
  });

  it('calls onSelect with the issue identifier when a card is clicked', async () => {
    const onSelect = vi.fn();
    render(
      <BoardColumn
        state="In Progress"
        issues={[makeIssue('ABC-99')]}
        isOver={false}
        onSelect={onSelect}
      />,
    );
    await userEvent.click(screen.getByTestId('issue-card'));
    expect(onSelect).toHaveBeenCalledWith('ABC-99');
  });
});
