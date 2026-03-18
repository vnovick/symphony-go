import { useRef, useState, useEffect } from 'react';

/**
 * Returns `value` unchanged while it is non-empty.
 * When `value` becomes empty, retains the last non-empty value for `retainMs`
 * milliseconds before clearing — prevents flicker during brief SSE reconnects.
 */
export function useStableValue<T extends readonly unknown[] | unknown[]>(
  value: T,
  retainMs: number,
): T {
  const stableRef = useRef<T>(value);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [, forceUpdate] = useState(0);

  useEffect(() => {
    if (value.length > 0) {
      stableRef.current = value;
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    } else if (stableRef.current.length > 0 && timerRef.current === null) {
      timerRef.current = setTimeout(() => {
        stableRef.current = [] as unknown as T;
        timerRef.current = null;
        forceUpdate((n) => n + 1);
      }, retainMs);
    }
  }, [value, retainMs]);

  // Cleanup on unmount — prevents forceUpdate call on unmounted component.
  useEffect(() => {
    return () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current);
    };
  }, []);

  // eslint-disable-next-line react-hooks/refs
  return value.length > 0 ? value : stableRef.current;
}
