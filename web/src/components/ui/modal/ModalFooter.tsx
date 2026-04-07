interface ModalFooterProps {
  children: React.ReactNode;
  /** Alignment of footer buttons. Default: 'end'. */
  align?: 'start' | 'center' | 'end';
}

/**
 * Standard modal footer with consistent spacing and alignment.
 * Wraps action buttons (Cancel + primary action) in a flex row.
 */
export function ModalFooter({ children, align = 'end' }: ModalFooterProps) {
  const justifyClass =
    align === 'start' ? 'justify-start' : align === 'center' ? 'justify-center' : 'justify-end';
  return <div className={`mt-5 flex gap-2 ${justifyClass}`}>{children}</div>;
}
