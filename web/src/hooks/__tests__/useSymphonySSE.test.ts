import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSymphonySSE } from '../useSymphonySSE';
import { useSymphonyStore } from '../../store/symphonyStore';

vi.mock('../../store/symphonyStore', () => ({
  useSymphonyStore: vi.fn(),
}));

const mockSetSnapshot = vi.fn();
const mockSetSseConnected = vi.fn();

class MockEventSource {
  static instances: MockEventSource[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    MockEventSource.instances.push(this);
  }
  close = vi.fn();
  triggerOpen() {
    this.onopen?.();
  }
  triggerMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent);
  }
  triggerError() {
    this.onerror?.();
  }
}

beforeEach(() => {
  MockEventSource.instances = [];
  vi.useFakeTimers();
  (useSymphonyStore as unknown as ReturnType<typeof vi.fn>).mockImplementation(
    (
      sel: (s: {
        setSnapshot: typeof mockSetSnapshot;
        setSseConnected: typeof mockSetSseConnected;
      }) => unknown,
    ) => sel({ setSnapshot: mockSetSnapshot, setSseConnected: mockSetSseConnected }),
  );
  global.EventSource = MockEventSource as unknown as typeof EventSource;
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue({ running: [] }),
  });
});

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe('useSymphonySSE', () => {
  it('creates an EventSource connection on mount', () => {
    renderHook(() => {
      useSymphonySSE();
    });
    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0].url).toBe('/api/v1/events');
  });

  it('calls setSnapshot when SSE message arrives', () => {
    renderHook(() => {
      useSymphonySSE();
    });
    act(() => {
      MockEventSource.instances[0].triggerMessage({
        generatedAt: '2024-01-01T00:00:00Z',
        counts: { running: 0, retrying: 0, paused: 0 },
        running: [],
        retrying: [],
        paused: [],
        maxConcurrentAgents: 3,
        rateLimits: null,
      });
    });
    expect(mockSetSnapshot).toHaveBeenCalled();
  });

  it('calls setSseConnected(true) on SSE open', () => {
    renderHook(() => {
      useSymphonySSE();
    });
    act(() => {
      MockEventSource.instances[0].triggerOpen();
    });
    expect(mockSetSseConnected).toHaveBeenCalledWith(true);
  });

  it('starts polling fallback when SSE errors', async () => {
    renderHook(() => {
      useSymphonySSE();
    });
    // The hook starts polling on mount immediately, so fetch is called right away.
    // Trigger error to confirm polling continues after SSE failure.
    act(() => {
      MockEventSource.instances[0].triggerError();
    });
    // Advance just enough to allow the initial poll promise to resolve.
    await vi.advanceTimersByTimeAsync(100);
    expect(global.fetch).toHaveBeenCalledWith('/api/v1/state');
  });

  it('reconnects SSE after 5s when connection drops', async () => {
    renderHook(() => {
      useSymphonySSE();
    });
    act(() => {
      MockEventSource.instances[0].triggerError();
    });
    await vi.advanceTimersByTimeAsync(5000);
    expect(MockEventSource.instances).toHaveLength(2);
  });

  it('closes EventSource on unmount', () => {
    const { unmount } = renderHook(() => {
      useSymphonySSE();
    });
    const es = MockEventSource.instances[0];
    unmount();
    expect(es.close).toHaveBeenCalled();
  });
});
