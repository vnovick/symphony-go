import { useMemo } from 'react';
import { useItervoxStore } from '../store/itervoxStore';
import { useToastStore } from '../store/toastStore';
import { authedFetch } from '../auth/authedFetch';
import { UnauthorizedError } from '../auth/UnauthorizedError';

// Read refreshSnapshot from the store directly (not via selector) so
// the returned action functions have stable references across renders.
function getRefreshSnapshot() {
  return useItervoxStore.getState().refreshSnapshot;
}

function toastError(msg: string) {
  useToastStore.getState().addToast(msg, 'error');
}

async function settingsFetch(
  url: string,
  method: string,
  body?: unknown,
  errorLabel?: string,
): Promise<boolean> {
  try {
    const res = await authedFetch(url, {
      method,
      ...(body !== undefined
        ? { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
        : {}),
    });
    if (!res.ok) {
      toastError(errorLabel ?? 'Request failed. Check the server logs.');
      return false;
    }
    await getRefreshSnapshot()();
    return true;
  } catch (err) {
    if (err instanceof UnauthorizedError) return false; // AuthGate handles UI.
    toastError(errorLabel ? `Network error: ${errorLabel}` : 'Network error.');
    return false;
  }
}

// Module-level stable action objects — created once, never re-allocated.
const actions = {
  upsertProfile: async (
    name: string,
    command: string,
    backend?: string,
    prompt?: string,
  ): Promise<boolean> =>
    settingsFetch(
      `/api/v1/settings/profiles/${encodeURIComponent(name)}`,
      'PUT',
      { command, backend: backend ?? '', prompt: prompt ?? '' },
      `Failed to save profile "${name}".`,
    ),

  deleteProfile: async (name: string): Promise<boolean> =>
    settingsFetch(
      `/api/v1/settings/profiles/${encodeURIComponent(name)}`,
      'DELETE',
      undefined,
      `Failed to delete profile "${name}".`,
    ),

  updateTrackerStates: async (
    activeStates: string[],
    terminalStates: string[],
    completionState: string,
  ): Promise<boolean> =>
    settingsFetch(
      '/api/v1/settings/tracker/states',
      'PUT',
      { activeStates, terminalStates, completionState },
      'Failed to save tracker states.',
    ),

  setAutoClearWorkspace: async (enabled: boolean): Promise<boolean> =>
    settingsFetch(
      '/api/v1/settings/workspace/auto-clear',
      'POST',
      { enabled },
      'Failed to update auto-clear setting.',
    ),

  setProjectFilter: async (slugs: string[] | null): Promise<boolean> =>
    settingsFetch('/api/v1/projects/filter', 'PUT', { slugs }, 'Failed to update project filter.'),

  addSSHHost: async (host: string, description: string): Promise<boolean> =>
    settingsFetch(
      '/api/v1/settings/ssh-hosts',
      'POST',
      { host, description },
      `Failed to add SSH host "${host}".`,
    ),

  removeSSHHost: async (host: string): Promise<boolean> =>
    settingsFetch(
      `/api/v1/settings/ssh-hosts/${encodeURIComponent(host)}`,
      'DELETE',
      undefined,
      `Failed to remove SSH host "${host}".`,
    ),

  setDispatchStrategy: async (strategy: string): Promise<boolean> =>
    settingsFetch(
      '/api/v1/settings/dispatch-strategy',
      'PUT',
      { strategy },
      'Failed to update dispatch strategy.',
    ),

  bumpWorkers: async (delta: number): Promise<boolean> =>
    settingsFetch('/api/v1/settings/workers', 'POST', { delta }, 'Failed to update worker count.'),

  setReviewerConfig: async (profile: string, autoReview: boolean): Promise<boolean> =>
    settingsFetch(
      '/api/v1/settings/reviewer',
      'PUT',
      { profile, auto_review: autoReview },
      'Failed to update reviewer settings.',
    ),
};

export function useSettingsActions() {
  // Return a stable reference — actions is a module-level singleton.
  // useMemo with [] deps ensures the hook signature matches React conventions.
  return useMemo(() => actions, []);
}
