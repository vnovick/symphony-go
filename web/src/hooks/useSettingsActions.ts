import { useSymphonyStore } from '../store/symphonyStore';

export function useSettingsActions() {
  const refreshSnapshot = useSymphonyStore((s) => s.refreshSnapshot);
  const patchSnapshot = useSymphonyStore((s) => s.patchSnapshot);

  const upsertProfile = async (
    name: string,
    command: string,
    prompt?: string,
  ): Promise<boolean> => {
    try {
      const res = await fetch(`/api/v1/settings/profiles/${encodeURIComponent(name)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command, prompt: prompt ?? '' }),
      });
      if (res.ok) await refreshSnapshot();
      return res.ok;
    } catch {
      return false;
    }
  };

  const deleteProfile = async (name: string): Promise<boolean> => {
    try {
      const res = await fetch(`/api/v1/settings/profiles/${encodeURIComponent(name)}`, {
        method: 'DELETE',
      });
      if (res.ok) await refreshSnapshot();
      return res.ok;
    } catch {
      return false;
    }
  };

  const setAgentMode = async (mode: string): Promise<boolean> => {
    try {
      const res = await fetch('/api/v1/settings/agent-mode', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode }),
      });
      if (res.ok) patchSnapshot({ agentMode: mode });
      return res.ok;
    } catch {
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
      if (res.ok) {
        patchSnapshot({
          activeStates,
          terminalStates,
          completionState: completionState || undefined,
        });
      }
      return res.ok;
    } catch {
      return false;
    }
  };

  return { upsertProfile, deleteProfile, setAgentMode, updateTrackerStates };
}
