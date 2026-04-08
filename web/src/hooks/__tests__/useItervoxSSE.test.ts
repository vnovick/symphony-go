import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useItervoxSSE } from '../useItervoxSSE';
import { useItervoxStore } from '../../store/itervoxStore';

vi.mock('../../store/itervoxStore', () => ({
  useItervoxStore: vi.fn(),
}));

// Mock the auth event stream wrapper — useItervoxSSE goes through
// openAuthedEventStream instead of native EventSource now.
interface MockStreamHandle {
  url: string;
  onOpen?: () => void;
  onMessage: (msg: { event?: string; data: string; id?: string; retry?: number }) => void;
  onDisconnect?: () => void;
  close: ReturnType<typeof vi.fn>;
}

const streamHandles: MockStreamHandle[] = [];

vi.mock('../../auth/authedEventStream', () => ({
  openAuthedEventStream: (url: string, opts: Omit<MockStreamHandle, 'url' | 'close'>) => {
    const handle: MockStreamHandle = {
      url,
      onOpen: opts.onOpen,
      onMessage: opts.onMessage,
      onDisconnect: opts.onDisconnect,
      close: vi.fn(),
    };
    streamHandles.push(handle);
    return () => {
      handle.close();
    };
  },
}));

const mockSetSnapshot = vi.fn();
const mockSetSseConnected = vi.fn();

beforeEach(() => {
  streamHandles.length = 0;
  vi.useFakeTimers();
  (useItervoxStore as unknown as ReturnType<typeof vi.fn>).mockImplementation(
    (
      sel: (s: {
        setSnapshot: typeof mockSetSnapshot;
        setSseConnected: typeof mockSetSseConnected;
      }) => unknown,
    ) => sel({ setSnapshot: mockSetSnapshot, setSseConnected: mockSetSseConnected }),
  );
  Object.assign(useItervoxStore, {
    getState: () => ({ setSnapshot: mockSetSnapshot, setSseConnected: mockSetSseConnected }),
  });
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue({ running: [] }),
  });
});

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe('useItervoxSSE', () => {
  it('opens an authed event stream on mount', () => {
    renderHook(() => {
      useItervoxSSE();
    });
    expect(streamHandles).toHaveLength(1);
    expect(streamHandles[0].url).toBe('/api/v1/events');
  });

  it('calls setSnapshot when a valid SSE message arrives', () => {
    renderHook(() => {
      useItervoxSSE();
    });
    act(() => {
      streamHandles[0].onMessage({
        data: JSON.stringify({
          generatedAt: '2024-01-01T00:00:00Z',
          counts: { running: 0, retrying: 0, paused: 0 },
          running: [],
          retrying: [],
          paused: [],
          maxConcurrentAgents: 3,
          rateLimits: null,
        }),
      });
    });
    expect(mockSetSnapshot).toHaveBeenCalled();
  });

  it('calls setSseConnected(true) on stream open', () => {
    renderHook(() => {
      useItervoxSSE();
    });
    act(() => {
      streamHandles[0].onOpen?.();
    });
    expect(mockSetSseConnected).toHaveBeenCalledWith(true);
  });

  it('keeps polling fallback active when stream disconnects', async () => {
    renderHook(() => {
      useItervoxSSE();
    });
    // Hook starts polling on mount. Disconnect should also trigger polling.
    act(() => {
      streamHandles[0].onDisconnect?.();
    });
    // Allow initial poll promise to resolve.
    await vi.advanceTimersByTimeAsync(100);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/state',
      expect.objectContaining({ headers: expect.any(Headers) }),
    );
  });

  it('closes the stream on unmount', () => {
    const { unmount } = renderHook(() => {
      useItervoxSSE();
    });
    const handle = streamHandles[0];
    unmount();
    expect(handle.close).toHaveBeenCalled();
  });
});
