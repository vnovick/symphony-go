import { BrowserRouter as Router, Routes, Route, Outlet } from 'react-router';
import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useSymphonySSE } from './hooks/useSymphonySSE';
import { useLogStream } from './hooks/useLogStream';
import { useSymphonyStore } from './store/symphonyStore';
import { ISSUES_KEY } from './queries/issues';
import { logIdentifiersKey } from './queries/logs';
import IssueDetailSlide from './components/symphony/IssueDetailSlide';
import Toast from './components/common/Toast';
import { PageErrorBoundary } from './components/common/PageErrorBoundary';
import { NavLink } from './components/layout/NavLink';
import { ThemeToggle } from './components/ui/ThemeToggle/ThemeToggle';
import AppHeader from './layout/AppHeader';
import { useFocusTrap } from './hooks/useFocusTrap';
import { useMultiTabWarning } from './hooks/useMultiTabWarning';

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Logs = lazy(() => import('./pages/Logs'));
const Timeline = lazy(() => import('./pages/Timeline'));
const Settings = lazy(() => import('./pages/Settings'));
const NotFound = lazy(() => import('./pages/OtherPage/NotFound'));

function PageLoader() {
  return (
    <div className="flex h-64 items-center justify-center">
      <div
        className="h-6 w-6 animate-spin rounded-full border-2 border-current border-t-transparent"
      />
    </div>
  );
}

const NAV_ITEMS = [
  { to: '/',        icon: '◫', label: 'Dashboard' },
  { to: '/timeline',icon: '◷', label: 'Timeline'  },
  { to: '/logs',    icon: '⌨', label: 'Logs'      },
  { to: '/settings',icon: '⚙', label: 'Settings'  },
] as const;

function SidebarContent() {
  return (
    <>
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
    </>
  );
}

function AppShell() {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const drawerRef = useRef<HTMLDivElement>(null);

  useFocusTrap(drawerRef, mobileNavOpen);

  const closeMobileNav = useCallback(() => { setMobileNavOpen(false); }, []);

  useEffect(() => {
    if (!mobileNavOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        closeMobileNav();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => { document.removeEventListener('keydown', handleKeyDown); };
  }, [mobileNavOpen, closeMobileNav]);

  return (
    <div className="min-h-screen flex">
      {/* Desktop sidebar — hidden on mobile */}
      <aside
        className="hidden md:flex fixed left-0 top-0 bottom-0 w-16 flex-col items-center py-4 gap-2 border-r z-40 bg-theme-bg-soft border-theme-line"
      >
        <SidebarContent />
      </aside>

      {/* Mobile nav drawer — slides from left */}
      <div
        ref={drawerRef}
        role="dialog"
        aria-modal="true"
        aria-label="Navigation"
        className={`md:hidden fixed inset-0 z-50 transition-opacity duration-200 ${
          mobileNavOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
        }`}
      >
        <div
          className="absolute inset-0"
          style={{ background: 'rgba(0,0,0,0.5)' }}
          aria-label="Close navigation"
          role="button"
          tabIndex={0}
          onClick={closeMobileNav}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') closeMobileNav(); }}
        />
        <aside
          className={`absolute left-0 top-0 bottom-0 w-16 flex flex-col items-center py-4 gap-2 border-r bg-theme-bg-soft border-theme-line transition-transform duration-200 ${
            mobileNavOpen ? 'translate-x-0' : '-translate-x-full'
          }`}
        >
          <SidebarContent />
        </aside>
      </div>

      <main className="md:ml-16 flex-1 min-w-0 flex flex-col">
        <AppHeader onMenuClick={() => { setMobileNavOpen(true); }} />
        <div className="flex-1 p-3 md:p-6">
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
      void queryClient.invalidateQueries({ queryKey: logIdentifiersKey() });
    }
    prevRef.current = fingerprint;
  }, [fingerprint, queryClient]);
}

function AppWithSSE() {
  useSymphonySSE();
  useLogStream();
  useSnapshotInvalidation();
  useMultiTabWarning();

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
              <Suspense fallback={<PageLoader />}>
                <PageErrorBoundary>
                  <Dashboard />
                </PageErrorBoundary>
              </Suspense>
            }
          />
          <Route
            path="/timeline"
            element={
              <Suspense fallback={<PageLoader />}>
                <PageErrorBoundary>
                  <Timeline />
                </PageErrorBoundary>
              </Suspense>
            }
          />
          <Route
            path="/logs"
            element={
              <Suspense fallback={<PageLoader />}>
                <PageErrorBoundary>
                  <Logs />
                </PageErrorBoundary>
              </Suspense>
            }
          />
          <Route
            path="/settings"
            element={
              <Suspense fallback={<PageLoader />}>
                <PageErrorBoundary>
                  <Settings />
                </PageErrorBoundary>
              </Suspense>
            }
          />
        </Route>
        <Route
          path="*"
          element={
            <Suspense fallback={<PageLoader />}>
              <PageErrorBoundary>
                <NotFound />
              </PageErrorBoundary>
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
