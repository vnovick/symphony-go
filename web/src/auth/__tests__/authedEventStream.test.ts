import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTokenStore } from '../tokenStore';
import { useAuthStore } from '../authStore';

// Mock fetchEventSource BEFORE importing openAuthedEventStream so the module
// picks up the mocked version at import time.
type OnOpen = (res: Response) => Promise<void> | undefined;
type OnMessage = (msg: { event?: string; data: string; id?: string; retry?: number }) => void;
type OnClose = () => void;
type OnError = (err: unknown) => number | undefined;

interface MockFetchEventSourceArgs {
  url: string;
  signal?: AbortSignal;
  headers?: Record<string, string>;
  openWhenHidden?: boolean;
  onopen?: OnOpen;
  onmessage?: OnMessage;
  onclose?: OnClose;
  onerror?: OnError;
}

const mockFetchEventSource = vi.fn();

vi.mock('@microsoft/fetch-event-source', () => ({
  fetchEventSource: (url: string, init: Omit<MockFetchEventSourceArgs, 'url'>) => {
    mockFetchEventSource({ url, ...init });
    return Promise.resolve();
  },
}));

// Must import AFTER vi.mock so the mocked dependency is used.
// vi.mock is hoisted above imports by Vitest at runtime, so the order here is safe.
import { openAuthedEventStream } from '../authedEventStream';

beforeEach(() => {
  mockFetchEventSource.mockClear();
  useTokenStore.setState({ token: null });
  useAuthStore.setState({ status: 'authorized', rejectedReason: null });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('openAuthedEventStream', () => {
  it('calls fetchEventSource with the target URL', () => {
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    expect(mockFetchEventSource).toHaveBeenCalledTimes(1);
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    expect(call.url).toBe('/api/v1/events');
  });

  it('injects Authorization header when a token is stored', () => {
    useTokenStore.getState().setToken('abc123', false);
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    expect(call.headers).toEqual({ Authorization: 'Bearer abc123' });
  });

  it('sends an empty header object when no token is stored', () => {
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    expect(call.headers).toEqual({});
  });

  it('sets openWhenHidden: true to match native EventSource semantics', () => {
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    expect(call.openWhenHidden).toBe(true);
  });

  it('invokes opts.onOpen on a 200 response', async () => {
    const onOpen = vi.fn();
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined, onOpen });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    await call.onopen?.(new Response(null, { status: 200 }));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it('clears the token and marks unauthorized on 401 at open', async () => {
    useTokenStore.getState().setToken('bad', false);
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;

    await expect(async () => {
      await call.onopen?.(new Response(null, { status: 401 }));
    }).rejects.toThrow(/unauthorized/);

    expect(useTokenStore.getState().token).toBeNull();
    // markUnauthorized is queued as a microtask; drain it.
    await Promise.resolve();
    expect(useAuthStore.getState().status).toBe('needsToken');
  });

  it('throws on non-401 non-2xx open so the library retries', async () => {
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    await expect(async () => {
      await call.onopen?.(new Response(null, { status: 503 }));
    }).rejects.toThrow(/sse open failed: 503/);
  });

  it('forwards onmessage to the caller', () => {
    const onMessage = vi.fn();
    openAuthedEventStream('/api/v1/events', { onMessage });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    call.onmessage?.({ event: 'log', data: 'hello' });
    expect(onMessage).toHaveBeenCalledWith({ event: 'log', data: 'hello' });
  });

  it('calls opts.onDisconnect on close', () => {
    const onDisconnect = vi.fn();
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined, onDisconnect });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    call.onclose?.();
    expect(onDisconnect).toHaveBeenCalledTimes(1);
  });

  it('returns exponential backoff delays from onerror', () => {
    const onDisconnect = vi.fn();
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined, onDisconnect });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;

    // First error → 1000ms (base)
    const d1 = call.onerror?.(new Error('boom'));
    // Second error → 2000ms
    const d2 = call.onerror?.(new Error('boom'));
    // Third error → 4000ms
    const d3 = call.onerror?.(new Error('boom'));

    expect(d1).toBe(1000);
    expect(d2).toBe(2000);
    expect(d3).toBe(4000);
    // onDisconnect fires on every transient error.
    expect(onDisconnect).toHaveBeenCalledTimes(3);
  });

  it('caps backoff at 15s', () => {
    openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    // 20 consecutive errors — backoff must not exceed 15000ms
    let last: number | undefined;
    for (let i = 0; i < 20; i++) {
      last = call.onerror?.(new Error('boom'));
    }
    expect(last).toBe(15000);
  });

  it('returns a closer function that aborts the underlying fetch', () => {
    const close = openAuthedEventStream('/api/v1/events', { onMessage: () => undefined });
    const call = mockFetchEventSource.mock.calls[0][0] as MockFetchEventSourceArgs;
    expect(call.signal?.aborted).toBe(false);
    close();
    expect(call.signal?.aborted).toBe(true);
  });
});
