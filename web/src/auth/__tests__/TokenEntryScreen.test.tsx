import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TokenEntryScreen } from '../TokenEntryScreen';
import { useTokenStore } from '../tokenStore';
import { useAuthStore } from '../authStore';

beforeEach(() => {
  useTokenStore.setState({ token: null });
  useAuthStore.setState({ status: 'needsToken', rejectedReason: null });
  sessionStorage.clear();
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('TokenEntryScreen', () => {
  it('renders the form with a password input', () => {
    render(<TokenEntryScreen />);
    expect(screen.getByLabelText(/API token/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('shows an inline error when submitted empty', async () => {
    const user = userEvent.setup();
    render(<TokenEntryScreen />);
    await user.click(screen.getByRole('button', { name: /sign in/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/paste your API token/i);
  });

  it('validates the token against /api/v1/state, stores it, and flips auth status on success', async () => {
    global.fetch = vi
      .fn()
      .mockResolvedValue(new Response('{}', { status: 200 })) as unknown as typeof fetch;

    const user = userEvent.setup();
    render(<TokenEntryScreen />);

    await user.type(screen.getByLabelText(/API token/i), 'good-token');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe('authorized');
    });
    expect(useTokenStore.getState().token).toBe('good-token');
    // Default: session storage, not persistent.
    expect(sessionStorage.getItem('itervox.apiToken')).toBe('good-token');
    expect(localStorage.getItem('itervox.apiToken.persistent')).toBeNull();
  });

  it('uses localStorage when "Remember" is checked', async () => {
    global.fetch = vi
      .fn()
      .mockResolvedValue(new Response('{}', { status: 200 })) as unknown as typeof fetch;

    const user = userEvent.setup();
    render(<TokenEntryScreen />);

    await user.type(screen.getByLabelText(/API token/i), 'sticky-token');
    await user.click(screen.getByLabelText(/Remember on this device/i));
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(localStorage.getItem('itervox.apiToken.persistent')).toBe('sticky-token');
    });
    expect(sessionStorage.getItem('itervox.apiToken')).toBeNull();
  });

  it('shows a 401-specific error and does not store the token on rejection', async () => {
    global.fetch = vi
      .fn()
      .mockResolvedValue(new Response('nope', { status: 401 })) as unknown as typeof fetch;

    const user = userEvent.setup();
    render(<TokenEntryScreen />);

    await user.type(screen.getByLabelText(/API token/i), 'bad-token');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/rejected/i);
    expect(useTokenStore.getState().token).toBeNull();
    expect(useAuthStore.getState().status).toBe('needsToken');
  });

  it('shows a network-error message when fetch rejects', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('disconnected')) as unknown as typeof fetch;

    const user = userEvent.setup();
    render(<TokenEntryScreen />);

    await user.type(screen.getByLabelText(/API token/i), 'anything');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/network error/i);
  });
});
