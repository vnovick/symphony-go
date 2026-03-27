import { create } from 'zustand';

type ViewMode = 'board' | 'list' | 'agents';

interface UIState {
  // Dashboard preferences (ZUSTAND-3) — persist across navigation
  dashboardViewMode: ViewMode;
  dashboardSearch: string;
  dashboardStateFilter: string;
  dashboardSearchVisible: boolean;

  // Accordion expansion (ZUSTAND-6) — persist across re-renders
  expandedRunningId: string | null;
  expandedPausedId: string | null;
}

interface UIActions {
  setDashboardViewMode: (mode: ViewMode) => void;
  setDashboardSearch: (search: string) => void;
  setDashboardStateFilter: (filter: string) => void;
  setDashboardSearchVisible: (visible: boolean) => void;
  setExpandedRunningId: (id: string | null) => void;
  setExpandedPausedId: (id: string | null) => void;
}

export const useUIStore = create<UIState & UIActions>((set) => ({
  dashboardViewMode: 'board',
  dashboardSearch: '',
  dashboardStateFilter: 'all',
  dashboardSearchVisible: false,
  expandedRunningId: null,
  expandedPausedId: null,

  setDashboardViewMode: (dashboardViewMode) => set({ dashboardViewMode }),
  setDashboardSearch: (dashboardSearch) => set({ dashboardSearch }),
  setDashboardStateFilter: (dashboardStateFilter) => set({ dashboardStateFilter }),
  setDashboardSearchVisible: (dashboardSearchVisible) => set({ dashboardSearchVisible }),
  setExpandedRunningId: (expandedRunningId) => set({ expandedRunningId }),
  setExpandedPausedId: (expandedPausedId) => set({ expandedPausedId }),
}));
