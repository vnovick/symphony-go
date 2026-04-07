import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { AgentProfileSelector } from '../AgentProfileSelector';

describe('AgentProfileSelector', () => {
  const profiles = ['reviewer', 'coder', 'planner'];

  it('renders nothing when no profiles available', () => {
    const { container } = render(
      <AgentProfileSelector value="" availableProfiles={[]} onChange={vi.fn()} />,
    );
    expect(container.innerHTML).toBe('');
  });

  it('renders dropdown with profile options', () => {
    render(<AgentProfileSelector value="" availableProfiles={profiles} onChange={vi.fn()} />);
    const select = screen.getByRole('combobox');
    expect(select).toBeInTheDocument();
    expect(screen.getByText('reviewer')).toBeInTheDocument();
    expect(screen.getByText('coder')).toBeInTheDocument();
    expect(screen.getByText('planner')).toBeInTheDocument();
  });

  it('shows Default option for empty value', () => {
    render(<AgentProfileSelector value="" availableProfiles={profiles} onChange={vi.fn()} />);
    expect(screen.getByText('Default')).toBeInTheDocument();
  });

  it('shows Agent label by default', () => {
    render(<AgentProfileSelector value="" availableProfiles={profiles} onChange={vi.fn()} />);
    expect(screen.getByText('Agent:')).toBeInTheDocument();
  });

  it('hides label when showLabel is false', () => {
    render(
      <AgentProfileSelector
        value=""
        availableProfiles={profiles}
        onChange={vi.fn()}
        showLabel={false}
      />,
    );
    expect(screen.queryByText('Agent:')).toBeNull();
    // dropdown still renders
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('fires onChange when selection changes', () => {
    const onChange = vi.fn();
    render(<AgentProfileSelector value="" availableProfiles={profiles} onChange={onChange} />);
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'coder' } });
    expect(onChange).toHaveBeenCalledWith('coder');
  });

  it('renders with sm size variant', () => {
    render(
      <AgentProfileSelector value="" availableProfiles={profiles} onChange={vi.fn()} size="sm" />,
    );
    const select = screen.getByRole('combobox');
    expect(select.className).toContain('text-[10px]');
  });

  it('renders with md size variant', () => {
    render(
      <AgentProfileSelector value="" availableProfiles={profiles} onChange={vi.fn()} size="md" />,
    );
    const select = screen.getByRole('combobox');
    expect(select.className).toContain('text-xs');
  });

  it('reflects the selected value', () => {
    render(
      <AgentProfileSelector value="planner" availableProfiles={profiles} onChange={vi.fn()} />,
    );
    expect(screen.getByRole('combobox')).toHaveValue('planner');
  });
});
