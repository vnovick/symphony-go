// Label resolver for the dashboard view-mode toggle. Notifications mode
// renders a count badge when total > 0. Extracted from Dashboard/index.tsx
// to keep that file under its size-budget cap.

export type DashboardViewMode = 'board' | 'list' | 'agents' | 'notifications';

export function viewModeLabel(mode: DashboardViewMode, notificationsTotal: number): string {
  switch (mode) {
    case 'board':
      return 'Board';
    case 'list':
      return 'List';
    case 'agents':
      return 'Agents';
    case 'notifications':
      return notificationsTotal > 0
        ? `Notifications · ${String(notificationsTotal)}`
        : 'Notifications';
  }
}
