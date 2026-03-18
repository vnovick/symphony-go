import { BrowserRouter as Router, Routes, Route } from 'react-router';
import AppLayout from './layout/AppLayout';
import { lazy, Suspense, useEffect } from 'react';
import { useSymphonySSE } from './hooks/useSymphonySSE';
import { useLogStream } from './hooks/useLogStream';
import { useSymphonyStore } from './store/symphonyStore';
import IssueDetailModal from './components/symphony/IssueDetailModal';

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Logs = lazy(() => import('./pages/Logs'));
const Timeline = lazy(() => import('./pages/Timeline'));
const Settings = lazy(() => import('./pages/Settings'));

function AppWithSSE() {
  useSymphonySSE();
  useLogStream(); // stream raw log lines into the store for real-time subagent visibility

  // Fetch snapshot once on mount as a fallback for the SSE handshake window.
  // Ongoing updates are delivered via SSE (useSymphonySSE hook above).
  const refreshSnapshot = useSymphonyStore((s) => s.refreshSnapshot);
  useEffect(() => {
    void refreshSnapshot();
  }, [refreshSnapshot]);

  return (
    <>
      <Routes>
        <Route element={<AppLayout />}>
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
              <Dashboard />
            </Suspense>
          }
        />
      </Routes>
      <IssueDetailModal />
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
