import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useFocusTrap } from '../useFocusTrap';

describe('useFocusTrap', () => {
  let container: HTMLDivElement;
  let rafCallbacks: Array<FrameRequestCallback>;
  let originalRaf: typeof requestAnimationFrame;
  let originalCaf: typeof cancelAnimationFrame;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    rafCallbacks = [];

    // Mock requestAnimationFrame to run synchronously in tests
    originalRaf = globalThis.requestAnimationFrame;
    originalCaf = globalThis.cancelAnimationFrame;
    globalThis.requestAnimationFrame = vi.fn((cb: FrameRequestCallback) => {
      rafCallbacks.push(cb);
      return rafCallbacks.length;
    }) as any;
    globalThis.cancelAnimationFrame = vi.fn();
  });

  afterEach(() => {
    document.body.removeChild(container);
    globalThis.requestAnimationFrame = originalRaf;
    globalThis.cancelAnimationFrame = originalCaf;
  });

  function flushRaf() {
    for (const cb of rafCallbacks) {
      cb(0);
    }
    rafCallbacks = [];
  }

  it('focuses the first focusable element when isOpen is true', () => {
    const btn1 = document.createElement('button');
    btn1.textContent = 'First';
    const btn2 = document.createElement('button');
    btn2.textContent = 'Second';
    container.appendChild(btn1);
    container.appendChild(btn2);

    const ref = { current: container };
    renderHook(() => {
      useFocusTrap(ref, true);
    });
    flushRaf();

    expect(document.activeElement).toBe(btn1);
  });

  it('does not focus anything when isOpen is false', () => {
    const btn = document.createElement('button');
    btn.textContent = 'Click me';
    container.appendChild(btn);

    // Focus something else first
    document.body.focus();

    const ref = { current: container };
    renderHook(() => {
      useFocusTrap(ref, false);
    });
    flushRaf();

    expect(document.activeElement).not.toBe(btn);
  });

  it('makes container itself focusable when no focusable children exist', () => {
    const span = document.createElement('span');
    span.textContent = 'Not focusable';
    container.appendChild(span);

    const ref = { current: container };
    renderHook(() => {
      useFocusTrap(ref, true);
    });
    flushRaf();

    expect(container.getAttribute('tabindex')).toBe('-1');
    expect(document.activeElement).toBe(container);
  });

  it('traps Tab at last element to wrap to first', () => {
    const btn1 = document.createElement('button');
    btn1.textContent = 'First';
    const btn2 = document.createElement('button');
    btn2.textContent = 'Last';
    container.appendChild(btn1);
    container.appendChild(btn2);

    const ref = { current: container };
    renderHook(() => {
      useFocusTrap(ref, true);
    });
    flushRaf();

    // Focus the last element
    btn2.focus();
    expect(document.activeElement).toBe(btn2);

    // Simulate Tab keydown
    const tabEvent = new KeyboardEvent('keydown', {
      key: 'Tab',
      shiftKey: false,
      bubbles: true,
      cancelable: true,
    });
    const preventDefaultSpy = vi.spyOn(tabEvent, 'preventDefault');
    document.dispatchEvent(tabEvent);

    expect(preventDefaultSpy).toHaveBeenCalled();
    expect(document.activeElement).toBe(btn1);
  });

  it('traps Shift+Tab at first element to wrap to last', () => {
    const btn1 = document.createElement('button');
    btn1.textContent = 'First';
    const btn2 = document.createElement('button');
    btn2.textContent = 'Last';
    container.appendChild(btn1);
    container.appendChild(btn2);

    const ref = { current: container };
    renderHook(() => {
      useFocusTrap(ref, true);
    });
    flushRaf();

    // First element should be focused
    expect(document.activeElement).toBe(btn1);

    // Simulate Shift+Tab keydown
    const shiftTabEvent = new KeyboardEvent('keydown', {
      key: 'Tab',
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    });
    const preventDefaultSpy = vi.spyOn(shiftTabEvent, 'preventDefault');
    document.dispatchEvent(shiftTabEvent);

    expect(preventDefaultSpy).toHaveBeenCalled();
    expect(document.activeElement).toBe(btn2);
  });

  it('restores focus to previously focused element on close', () => {
    const outsideBtn = document.createElement('button');
    outsideBtn.textContent = 'Outside';
    document.body.appendChild(outsideBtn);
    outsideBtn.focus();
    expect(document.activeElement).toBe(outsideBtn);

    const innerBtn = document.createElement('button');
    innerBtn.textContent = 'Inside';
    container.appendChild(innerBtn);

    const ref = { current: container };
    const { rerender } = renderHook(
      ({ isOpen }) => {
        useFocusTrap(ref, isOpen);
      },
      { initialProps: { isOpen: true } },
    );
    flushRaf();

    expect(document.activeElement).toBe(innerBtn);

    // Close the trap
    rerender({ isOpen: false });

    expect(document.activeElement).toBe(outsideBtn);

    document.body.removeChild(outsideBtn);
  });

  it('removes keydown listener when isOpen becomes false', () => {
    const btn1 = document.createElement('button');
    container.appendChild(btn1);

    const removeListenerSpy = vi.spyOn(document, 'removeEventListener');

    const ref = { current: container };
    const { rerender } = renderHook(
      ({ isOpen }) => {
        useFocusTrap(ref, isOpen);
      },
      { initialProps: { isOpen: true } },
    );
    flushRaf();

    rerender({ isOpen: false });

    expect(removeListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function));
    removeListenerSpy.mockRestore();
  });
});
