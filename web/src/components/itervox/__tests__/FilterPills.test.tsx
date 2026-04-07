import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { FilterPills, type FilterPill } from '../FilterPills';

const pills: FilterPill[] = [
  { id: 'all', label: 'All Issues', states: [] },
  { id: 'active', label: 'Active', states: ['In Progress'] },
  { id: 'done', label: 'Done', states: ['Done', 'Closed'] },
];

describe('FilterPills', () => {
  it('renders all pills', () => {
    render(<FilterPills pills={pills} activeId="all" onChange={vi.fn()} />);
    expect(screen.getByText('All Issues')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Done')).toBeInTheDocument();
  });

  it('highlights the active pill', () => {
    render(<FilterPills pills={pills} activeId="active" onChange={vi.fn()} />);
    const activeButton = screen.getByText('Active');
    expect(activeButton.className).toContain('bg-theme-accent');
    expect(activeButton.className).toContain('text-white');

    const inactiveButton = screen.getByText('All Issues');
    expect(inactiveButton.className).toContain('bg-theme-bg-soft');
  });

  it('calls onChange when a pill is clicked', () => {
    const onChange = vi.fn();
    render(<FilterPills pills={pills} activeId="all" onChange={onChange} />);
    fireEvent.click(screen.getByText('Done'));
    expect(onChange).toHaveBeenCalledWith('done');
  });

  it('renders nothing when pills array is empty', () => {
    const { container } = render(<FilterPills pills={[]} activeId="all" onChange={vi.fn()} />);
    expect(container.querySelectorAll('button')).toHaveLength(0);
  });

  it('renders all pills as buttons', () => {
    render(<FilterPills pills={pills} activeId="all" onChange={vi.fn()} />);
    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(3);
  });

  it('does not highlight non-active pills with accent class', () => {
    render(<FilterPills pills={pills} activeId="all" onChange={vi.fn()} />);
    const doneButton = screen.getByText('Done');
    expect(doneButton.className).not.toContain('bg-theme-accent');
  });
});
