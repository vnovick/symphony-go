import { useEffect, useRef, type RefObject } from 'react';

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'textarea:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

/**
 * Traps keyboard focus within a container while `isOpen` is true.
 *
 * On open:  saves `document.activeElement`, focuses the first focusable child.
 * On Tab at the last element:  wraps to the first.
 * On Shift+Tab at the first element:  wraps to the last.
 * On close: restores focus to the previously focused element.
 */
export function useFocusTrap(containerRef: RefObject<HTMLElement | null>, isOpen: boolean): void {
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!isOpen) return;

    // Save the element that was focused before the trap opened.
    previouslyFocusedRef.current = document.activeElement as HTMLElement | null;

    const container = containerRef.current;
    if (!container) return;

    // Small delay so the DOM has rendered focusable children.
    const rafId = requestAnimationFrame(() => {
      const focusable = container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
      if (focusable.length > 0) {
        focusable[0].focus();
      } else {
        // If no focusable child exists, make the container itself focusable.
        container.setAttribute('tabindex', '-1');
        container.focus();
      }
    });

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key !== 'Tab') return;
      if (!container) return;

      const focusable = container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];

      if (event.shiftKey) {
        if (document.activeElement === first) {
          event.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          event.preventDefault();
          first.focus();
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown);

    return () => {
      cancelAnimationFrame(rafId);
      document.removeEventListener('keydown', handleKeyDown);

      // Restore focus to the element that was focused before the trap.
      if (
        previouslyFocusedRef.current &&
        typeof previouslyFocusedRef.current.focus === 'function'
      ) {
        previouslyFocusedRef.current.focus();
      }
      previouslyFocusedRef.current = null;
    };
  }, [isOpen, containerRef]);
}
