import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MetricsGrid } from '../MetricsGrid';
import type { Metric } from '../MetricsGrid';

const sampleMetrics: Metric[] = [
  { label: 'Running', value: 3, status: 'live' },
  { label: 'Paused', value: 0, status: 'idle' },
  { label: 'Retrying', value: 1, status: 'warning' },
  { label: 'Capacity', value: '3/5', status: 'success' },
];

describe('MetricsGrid', () => {
  it('renders all metric labels', () => {
    render(<MetricsGrid metrics={sampleMetrics} />);
    expect(screen.getByText('Running')).toBeInTheDocument();
    expect(screen.getByText('Paused')).toBeInTheDocument();
    expect(screen.getByText('Retrying')).toBeInTheDocument();
    expect(screen.getByText('Capacity')).toBeInTheDocument();
  });

  it('renders metric values', () => {
    render(<MetricsGrid metrics={sampleMetrics} />);
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('3/5')).toBeInTheDocument();
  });

  it('defaults to 4 columns', () => {
    const { container } = render(<MetricsGrid metrics={sampleMetrics} />);
    expect(container.firstChild).toHaveAttribute('data-columns', '4');
  });

  it('applies 2 columns when specified', () => {
    const { container } = render(<MetricsGrid metrics={sampleMetrics} columns={2} />);
    expect(container.firstChild).toHaveAttribute('data-columns', '2');
  });

  it('applies 3 columns when specified', () => {
    const { container } = render(<MetricsGrid metrics={sampleMetrics} columns={3} />);
    expect(container.firstChild).toHaveAttribute('data-columns', '3');
  });

  it('renders LiveIndicator for each metric', () => {
    render(<MetricsGrid metrics={sampleMetrics} />);
    // Each metric cell should have a status dot
    const dots = document.querySelectorAll('[data-status]');
    expect(dots.length).toBe(sampleMetrics.length);
  });
});
