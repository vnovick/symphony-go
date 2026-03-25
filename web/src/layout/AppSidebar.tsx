/**
 * AppSidebar — icon-only navigation sidebar.
 *
 * NOTE: The primary sidebar is now rendered directly in AppShell (App.tsx).
 * This file is retained for any legacy references and as a standalone component
 * that can be imported if needed.
 */
import { NavLink } from '../components/layout/NavLink';
import { ThemeToggle } from '../components/ui/ThemeToggle/ThemeToggle';

const NAV_ITEMS = [
  { to: '/', icon: '◫', label: 'Dashboard' },
  { to: '/timeline', icon: '◷', label: 'Timeline' },
  { to: '/logs', icon: '⌨', label: 'Logs' },
  { to: '/settings', icon: '⚙', label: 'Settings' },
] as const;

const AppSidebar: React.FC = () => {
  return (
    <aside
      className="fixed left-0 top-0 bottom-0 w-16 flex flex-col items-center py-4 gap-2 border-r z-40"
      style={{
        background: 'var(--bg-soft)',
        borderColor: 'var(--line)',
      }}
    >
      <div
        className="w-9 h-9 rounded-lg mb-2 flex items-center justify-center text-white font-bold text-sm"
        style={{ background: 'var(--gradient-accent)' }}
        aria-label="Symphony"
      >
        S
      </div>

      <nav className="flex flex-col gap-1 flex-1">
        {NAV_ITEMS.map((item) => (
          <NavLink key={item.to} to={item.to} icon={item.icon} label={item.label} />
        ))}
      </nav>

      <ThemeToggle />
    </aside>
  );
};

export default AppSidebar;
