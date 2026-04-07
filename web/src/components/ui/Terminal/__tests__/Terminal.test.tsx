import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Terminal } from '../Terminal';
import type { LogEntry } from '../Terminal';

const makeEntry = (level: LogEntry['level'], message: string, ts = 1000): LogEntry => ({
  ts,
  level,
  message,
});

describe('Terminal', () => {
  it('renders log messages', () => {
    render(<Terminal entries={[makeEntry('info', 'Hello world')]} />);
    expect(screen.getByText('Hello world')).toBeInTheDocument();
  });

  it('renders multiple entries', () => {
    render(<Terminal entries={[makeEntry('info', 'First'), makeEntry('warn', 'Second')]} />);
    expect(screen.getByText('First')).toBeInTheDocument();
    expect(screen.getByText('Second')).toBeInTheDocument();
  });

  it('applies info level color class', () => {
    render(<Terminal entries={[makeEntry('info', 'info msg')]} />);
    const el = screen.getByText('info msg');
    expect(el).toHaveAttribute('data-level', 'info');
  });

  it('applies warn level color class', () => {
    render(<Terminal entries={[makeEntry('warn', 'warn msg')]} />);
    expect(screen.getByText('warn msg')).toHaveAttribute('data-level', 'warn');
  });

  it('applies error level color class', () => {
    render(<Terminal entries={[makeEntry('error', 'err msg')]} />);
    expect(screen.getByText('err msg')).toHaveAttribute('data-level', 'error');
  });

  it('applies action level color class', () => {
    render(<Terminal entries={[makeEntry('action', 'act msg')]} />);
    expect(screen.getByText('act msg')).toHaveAttribute('data-level', 'action');
  });

  it('applies subagent level color class', () => {
    render(<Terminal entries={[makeEntry('subagent', 'sub msg')]} />);
    expect(screen.getByText('sub msg')).toHaveAttribute('data-level', 'subagent');
  });

  it('shows timestamp when showTime is true', () => {
    render(<Terminal entries={[makeEntry('info', 'msg', 0)]} showTime />);
    // timestamp row should contain the time formatted
    expect(screen.getByTestId('terminal-time-0')).toBeInTheDocument();
  });

  it('hides timestamp when showTime is false', () => {
    render(<Terminal entries={[makeEntry('info', 'msg', 0)]} showTime={false} />);
    expect(screen.queryByTestId('terminal-time-0')).not.toBeInTheDocument();
  });

  it('renders empty state without crashing', () => {
    const { container } = render(<Terminal entries={[]} />);
    expect(container.firstChild).toBeInTheDocument();
  });
});
