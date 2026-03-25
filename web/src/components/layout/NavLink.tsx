import { Link, useMatch, useResolvedPath } from 'react-router';

interface NavLinkProps {
  to: string;
  icon: string;
  label: string;
}

export function NavLink({ to, icon, label }: NavLinkProps) {
  const resolved = useResolvedPath(to);
  const match = useMatch({ path: resolved.pathname, end: true });

  return (
    <Link
      to={to}
      aria-label={label}
      title={label}
      data-active={match ? 'true' : undefined}
      style={
        match
          ? { background: 'var(--accent-soft)', color: 'var(--accent-strong)', border: '1px solid var(--accent)' }
          : { color: 'var(--text-secondary)', border: '1px solid transparent' }
      }
      className="flex items-center justify-center w-10 h-10 rounded-[var(--radius-md)] transition-colors hover:bg-[var(--bg-elevated)] hover:text-[var(--text)]"
    >
      <span aria-hidden="true">{icon}</span>
    </Link>
  );
}
