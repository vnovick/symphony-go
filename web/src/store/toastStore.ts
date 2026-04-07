import { create } from 'zustand';
import { TOAST_DISMISS_MS } from '../utils/timings';

export interface ToastItem {
  id: string;
  message: string;
  variant: 'error' | 'success' | 'info';
}

interface ToastState {
  toasts: ToastItem[];
  // Internal map of auto-dismiss timers keyed by toast id.
  // Kept in the store so removeToast can cancel the pending timer, preventing
  // a stale setTimeout callback from running after the toast was manually dismissed.
  _timers: Map<string, ReturnType<typeof setTimeout>>;
  addToast: (message: string, variant?: ToastItem['variant']) => void;
  removeToast: (id: string) => void;
}

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  _timers: new Map(),

  addToast: (message, variant = 'error') => {
    const id = Math.random().toString(36).slice(2);
    const timer = setTimeout(() => {
      get().removeToast(id);
    }, TOAST_DISMISS_MS);
    set((state) => {
      const timers = new Map(state._timers);
      timers.set(id, timer);
      return { toasts: [...state.toasts, { id, message, variant }], _timers: timers };
    });
  },

  removeToast: (id) => {
    set((state) => {
      const timer = state._timers.get(id);
      if (timer !== undefined) clearTimeout(timer);
      const timers = new Map(state._timers);
      timers.delete(id);
      return { toasts: state.toasts.filter((t) => t.id !== id), _timers: timers };
    });
  },
}));
