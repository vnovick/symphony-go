import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import useGoBack from '../useGoBack';

const mockNavigate = vi.fn();

vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}));

beforeEach(() => {
  mockNavigate.mockReset();
});

describe('useGoBack', () => {
  it('navigates to -1 when history index is greater than 0', () => {
    Object.defineProperty(window, 'history', {
      writable: true,
      value: { state: { idx: 2 } },
    });

    const { result } = renderHook(() => useGoBack());

    act(() => {
      result.current();
    });

    expect(mockNavigate).toHaveBeenCalledWith(-1);
  });

  it('navigates to "/" when history index is 0', () => {
    Object.defineProperty(window, 'history', {
      writable: true,
      value: { state: { idx: 0 } },
    });

    const { result } = renderHook(() => useGoBack());

    act(() => {
      result.current();
    });

    expect(mockNavigate).toHaveBeenCalledWith('/');
  });

  it('navigates to "/" when history state is null (no idx)', () => {
    Object.defineProperty(window, 'history', {
      writable: true,
      value: { state: null },
    });

    const { result } = renderHook(() => useGoBack());

    act(() => {
      result.current();
    });

    expect(mockNavigate).toHaveBeenCalledWith('/');
  });
});
