import { useSymphonyStore } from '../store/symphonyStore';
import { useToastStore } from '../store/toastStore';

export function useSettingsActions() {
  const refreshSnapshot = useSymphonyStore((s) => s.refreshSnapshot);

  // Surface HTTP and network failures as toasts so the user is not left
  // wondering why nothing changed (FE-R10-5).
  // addToast signature: (message: string, variant?: 'error'|'success'|'info')
  const toastError = (msg: string) =>
    useToastStore.getState().addToast(msg, 'error');

  const upsertProfile = async (
    name: string,
    command: string,
    backend?: string,
    prompt?: string,
  ): Promise<boolean> => {
    try {
      const res = await fetch(`/api/v1/settings/profiles/${encodeURIComponent(name)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command, backend: backend ?? '', prompt: prompt ?? '' }),
      });
      if (!res.ok) {
        toastError(`Failed to save profile "${name}". Check the server logs.`);
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError(`Network error saving profile "${name}".`);
      return false;
    }
  };

  const deleteProfile = async (name: string): Promise<boolean> => {
    try {
      const res = await fetch(`/api/v1/settings/profiles/${encodeURIComponent(name)}`, {
        method: 'DELETE',
      });
      if (!res.ok) {
        toastError(`Failed to delete profile "${name}". Check the server logs.`);
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError(`Network error deleting profile "${name}".`);
      return false;
    }
  };

  const updateTrackerStates = async (
    activeStates: string[],
    terminalStates: string[],
    completionState: string,
  ): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/settings/tracker/states', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ activeStates, terminalStates, completionState }),
      });
      if (!res.ok) {
        toastError('Failed to save tracker states. Check the server logs.');
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError('Network error saving tracker states.');
      return false;
    }
  };

  const setAutoClearWorkspace = async (enabled: boolean): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/settings/workspace/auto-clear', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      });
      if (!res.ok) {
        toastError('Failed to update auto-clear setting. Check the server logs.');
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError('Network error updating auto-clear setting.');
      return false;
    }
  };

  const setProjectFilter = async (slugs: string[] | null): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/projects/filter', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ slugs }),
      });
      if (!res.ok) {
        toastError('Failed to update project filter. Check the server logs.');
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError('Network error updating project filter.');
      return false;
    }
  };

  const addSSHHost = async (host: string, description: string): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/settings/ssh-hosts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ host, description }),
      });
      if (!res.ok) {
        toastError(`Failed to add SSH host "${host}". Check the server logs.`);
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError(`Network error adding SSH host "${host}".`);
      return false;
    }
  };

  const removeSSHHost = async (host: string): Promise<boolean> => {
    try {
      const res = await fetch(`/api/v1/settings/ssh-hosts/${encodeURIComponent(host)}`, {
        method: 'DELETE',
      });
      if (!res.ok) {
        toastError(`Failed to remove SSH host "${host}". Check the server logs.`);
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError(`Network error removing SSH host "${host}".`);
      return false;
    }
  };

  const setDispatchStrategy = async (strategy: string): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/settings/dispatch-strategy', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ strategy }),
      });
      if (!res.ok) {
        toastError('Failed to update dispatch strategy. Check the server logs.');
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError('Network error updating dispatch strategy.');
      return false;
    }
  };

  const bumpWorkers = async (delta: number): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/settings/workers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ delta }),
      });
      if (!res.ok) {
        toastError('Failed to update worker count. Check the server logs.');
        return false;
      }
      await refreshSnapshot();
      return true;
    } catch {
      toastError('Network error updating worker count.');
      return false;
    }
  };

  return {
    upsertProfile,
    deleteProfile,
    updateTrackerStates,
    setAutoClearWorkspace,
    setProjectFilter,
    addSSHHost,
    removeSSHHost,
    setDispatchStrategy,
    bumpWorkers,
  };
}
