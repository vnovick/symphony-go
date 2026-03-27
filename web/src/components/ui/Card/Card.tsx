import type { ReactNode } from 'react';

type CardVariant = 'default' | 'elevated' | 'outline';
type CardPadding = 'none' | 'sm' | 'md' | 'lg';

interface CardProps {
  children: ReactNode;
  variant?: CardVariant;
  padding?: CardPadding;
  className?: string;
  onClick?: React.MouseEventHandler<HTMLDivElement>;
}

const PADDING_CLASS: Record<CardPadding, string> = {
  none: '',
  sm: 'p-3',
  md: 'p-4',
  lg: 'p-6',
};

const VARIANT_STYLE: Record<CardVariant, React.CSSProperties> = {
  default: { background: 'var(--panel)', border: '1px solid var(--line)' },
  elevated: { background: 'var(--bg-elevated)', boxShadow: 'var(--shadow-md)' },
  outline: { background: 'transparent', border: '1px solid var(--line-strong)' },
};

function CardRoot({ children, variant = 'default', padding = 'md', className, onClick }: CardProps) {
  return (
    <div
      data-variant={variant}
      data-padding={padding}
      className={['rounded-[var(--radius-md)]', PADDING_CLASS[padding], className ?? ''].join(' ')}
      style={VARIANT_STYLE[variant]}
      onClick={onClick}
    >
      {children}
    </div>
  );
}

function Header({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={['pb-3 mb-3 border-b border-theme-line', className ?? ''].join(' ')}
    >
      {children}
    </div>
  );
}

function Body({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={className}>{children}</div>;
}

function Footer({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={['pt-3 mt-3 border-t border-theme-line', className ?? ''].join(' ')}
    >
      {children}
    </div>
  );
}

export const Card = Object.assign(CardRoot, { Header, Body, Footer });
