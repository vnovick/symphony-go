import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useStableValue } from '../useStableValue';

describe('useStableValue', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllTimers();
  });

  it('returns the current value when non-empty', () => {
    const items = [{ id: 'A' }, { id: 'B' }];
    const { result } = renderHook(({ v }) => useStableValue(v, 5000), {
      initialProps: { v: items },
    });
    expect(result.current).toEqual(items);
  });

  it('retains last non-empty value for retainMs when input becomes empty', () => {
    vi.useFakeTimers();
    const items = [{ id: 'A' }];
    const { result, rerender } = renderHook(({ v }) => useStableValue(v, 5000), {
      initialProps: { v: items },
    });

    // Clear the input
    rerender({ v: [] });
    // Value is still retained
    expect(result.current).toEqual(items);

    // Advance past retainMs
    act(() => {
      vi.advanceTimersByTime(5001);
    });
    expect(result.current).toEqual([]);
  });

  it('cancels the retain timer if a non-empty value arrives before timeout', () => {
    vi.useFakeTimers();
    const items = [{ id: 'A' }];
    const newItems = [{ id: 'B' }];
    const { result, rerender } = renderHook(({ v }) => useStableValue(v, 5000), {
      initialProps: { v: items },
    });

    rerender({ v: [] }); // starts 5s timer
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    rerender({ v: newItems }); // cancels timer, replaces retained value
    act(() => {
      vi.advanceTimersByTime(5001);
    }); // timer should be gone
    expect(result.current).toEqual(newItems);
  });

  it('returns empty array when initial value is empty', () => {
    const { result } = renderHook(() => useStableValue([], 5000));
    expect(result.current).toEqual([]);
  });
});
