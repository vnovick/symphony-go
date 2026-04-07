import type { RunningRow, RetryRow, HistoryRow, SSHHostInfo, ProfileDef } from '../types/schemas';

/**
 * Stable empty-array constants for Zustand selector fallbacks.
 * Module-level constants prevent new references on every render,
 * which avoids useSyncExternalStore re-render loops.
 */

export const EMPTY_RUNNING: RunningRow[] = [];
export const EMPTY_RETRYING: RetryRow[] = [];
export const EMPTY_HISTORY: HistoryRow[] = [];
export const EMPTY_STATES: string[] = [];
export const EMPTY_HOSTS: SSHHostInfo[] = [];
export const EMPTY_PAUSED: string[] = [];
export const EMPTY_PROFILES: string[] = [];
export const EMPTY_PROFILE_DEFS: Record<string, ProfileDef> = {};
