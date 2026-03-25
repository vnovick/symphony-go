import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LiveIndicator } from '../LiveIndicator';

describe('LiveIndicator', () => {
  it('renders without crashing', () => {
    const { container } = render(<LiveIndicator status="live" />);
    expect(container.firstChild).toBeInTheDocument();
  });

  it('renders an optional label', () => {
    render(<LiveIndicator status="live" label="Live" />);
    expect(screen.getByText('Live')).toBeInTheDocument();
  });

  it('does not render label when not provided', () => {
    const { container } = render(<LiveIndicator status="live" />);
    expect(container.querySelector('[data-label]')).not.toBeInTheDocument();
  });

  it('exposes data-status attribute for each status', () => {
    const statuses = ['live', 'success', 'warning', 'error', 'idle'] as const;
    for (const status of statuses) {
      const { container } = render(<LiveIndicator status={status} />);
      expect(container.firstChild).toHaveAttribute('data-status', status);
    }
  });

  it('applies sm size', () => {
    const { container } = render(<LiveIndicator status="live" size="sm" />);
    expect(container.firstChild).toHaveAttribute('data-size', 'sm');
  });

  it('applies lg size', () => {
    const { container } = render(<LiveIndicator status="live" size="lg" />);
    expect(container.firstChild).toHaveAttribute('data-size', 'lg');
  });

  it('shows pulse ring for live status', () => {
    const { container } = render(<LiveIndicator status="live" />);
    expect(container.querySelector('.animate-ping')).toBeInTheDocument();
  });

  it('does not show pulse ring for idle status', () => {
    const { container } = render(<LiveIndicator status="idle" />);
    expect(container.querySelector('.animate-ping')).not.toBeInTheDocument();
  });
});
