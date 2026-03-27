import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MobileMenuButton } from '../MobileMenuButton';

describe('MobileMenuButton', () => {
  it('renders with correct test id', () => {
    render(<MobileMenuButton onClick={vi.fn()} />);
    expect(screen.getByTestId('mobile-menu-button')).toBeInTheDocument();
  });

  it('has accessible label', () => {
    render(<MobileMenuButton onClick={vi.fn()} />);
    expect(screen.getByLabelText('Open navigation')).toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const onClick = vi.fn();
    render(<MobileMenuButton onClick={onClick} />);
    await userEvent.click(screen.getByTestId('mobile-menu-button'));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('renders an SVG icon', () => {
    const { container } = render(<MobileMenuButton onClick={vi.fn()} />);
    expect(container.querySelector('svg')).toBeInTheDocument();
  });
});
