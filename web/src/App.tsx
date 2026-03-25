import { BrowserRouter as Router, Routes, Route, Outlet } from 'react-router';
import { lazy, Suspense, useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useSymphonySSE } from './hooks/useSymphonySSE';
import { useLogStream } from './hooks/useLogStream';
import { useSymphonyStore } from './store/symphonyStore';
import { ISSUES_KEY } from './queries/issues';
import IssueDetailSlide from './components/symphony/IssueDetailSlide';
import Toast from './components/common/Toast';
import { NavLink } from './components/layout/NavLink';
import { ThemeToggle } from './components/ui/ThemeToggle/ThemeToggle';
import AppHeader from './layout/AppHeader';

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Logs = lazy(() => import('./pages/Logs'));
const Timeline = lazy(() => import('./pages/Timeline'));
const Settings = lazy(() => import('./pages/Settings'));
const NotFound = lazy(() => import('./pages/OtherPage/NotFound'));

const NAV_ITEMS = [
  { to: '/',        icon: '◫', label: 'Dashboard' },
  { to: '/timeline',icon: '◷', label: 'Timeline'  },
  { to: '/logs',    icon: '⌨', label: 'Logs'      },
  { to: '/settings',icon: '⚙', label: 'Settings'  },
] as const;

function AppShell() {
  return (
    <div className="min-h-screen flex">
      <aside
        className="fixed left-0 top-0 bottom-0 w-16 flex flex-col items-center py-4 gap-2 border-r z-40"
        style={{
          background: 'var(--bg-soft)',
          borderColor: 'var(--line)',
        }}
      >
        {/* Logo mark */}
        <div
          className="w-10 h-10 rounded-[var(--radius-md)] mb-2 flex items-center justify-center text-white font-bold text-base"
          style={{ background: 'var(--gradient-accent)', boxShadow: 'var(--shadow-glow)' }}
          aria-label="Symphony"
        >
          S
        </div>

        {/* Nav links */}
        <nav className="flex flex-col gap-1 flex-1">
          {NAV_ITEMS.map((item) => (
            <NavLink key={item.to} to={item.to} icon={item.icon} label={item.label} />
          ))}
        </nav>

        {/* Theme toggle pinned to bottom */}
        <ThemeToggle />
      </aside>

      <main className="ml-16 flex-1 min-w-0 flex flex-col">
        <AppHeader />
        <div className="flex-1 p-4 md:p-6">
          <Outlet />
        </div>
      </main>
    </div>
  );
}

/**
 * Invalidates the issues cache whenever the orchestrator's activity fingerprint
 * changes (sessions start, stop, pause, or enter the retry queue).
 * This bridges the real-time SSE snapshot to the issues list so the kanban
 * board refreshes within seconds of a state change — not on the 30s poll cycle.
 */
function useSnapshotInvalidation() {
  const queryClient = useQueryClient();
  // Subscribe to a minimal derived value to avoid invalidating on every SSE tick.
  // The fingerprint only changes when the count of active sessions changes.
  const fingerprint = useSymphonyStore((s) => {
    const snap = s.snapshot;
    if (!snap) return null;
    return `${String(snap.running.length)}:${String(snap.paused.length)}:${String(snap.retrying.length)}`;
  });
  const prevRef = useRef<string | null>(null);

  useEffect(() => {
    if (fingerprint === null) return; // no snapshot yet
    if (prevRef.current !== null && prevRef.current !== fingerprint) {
      void queryClient.invalidateQueries({ queryKey: ISSUES_KEY });
    }
    prevRef.current = fingerprint;
  }, [fingerprint, queryClient]);
}

function AppWithSSE() {
  useSymphonySSE();
  useLogStream();
  useSnapshotInvalidation();

  const refreshSnapshot = useSymphonyStore((s) => s.refreshSnapshot);
  useEffect(() => {
    void refreshSnapshot();
  }, [refreshSnapshot]);

  return (
    <>
      <Routes>
        <Route element={<AppShell />}>
          <Route
            index
            element={
              <Suspense fallback={null}>
                <Dashboard />
              </Suspense>
            }
          />
          <Route
            path="/timeline"
            element={
              <Suspense fallback={null}>
                <Timeline />
              </Suspense>
            }
          />
          <Route
            path="/logs"
            element={
              <Suspense fallback={null}>
                <Logs />
              </Suspense>
            }
          />
          <Route
            path="/settings"
            element={
              <Suspense fallback={null}>
                <Settings />
              </Suspense>
            }
          />
        </Route>
        <Route
          path="*"
          element={
            <Suspense fallback={null}>
              <NotFound />
            </Suspense>
          }
        />
      </Routes>
      <IssueDetailSlide />
      <Toast />
    </>
  );
}

export default function App() {
  return (
    <Router>
      <AppWithSSE />
    </Router>
  );
}
