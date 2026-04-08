import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ServerDownScreen } from '../ServerDownScreen';
import { useAuthStore } from '../authStore';

beforeEach(() => {
  useAuthStore.setState({ status: 'serverDown', rejectedReason: null });
});

describe('ServerDownScreen', () => {
  it('renders the "cannot reach daemon" message', () => {
    render(<ServerDownScreen onRetry={() => undefined} />);
    expect(screen.getByText(/can't reach the daemon/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('calls onRetry and resets auth status to unknown when clicked', async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(<ServerDownScreen onRetry={onRetry} />);

    await user.click(screen.getByRole('button', { name: /retry/i }));

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().status).toBe('unknown');
  });
});
