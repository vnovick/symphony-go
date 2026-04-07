import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SlidePanel } from '../SlidePanel';

describe('SlidePanel', () => {
  it('does not render content when closed', () => {
    render(
      <SlidePanel isOpen={false} onClose={() => undefined} title="Test">
        <p>Content</p>
      </SlidePanel>,
    );
    expect(screen.queryByText('Content')).not.toBeInTheDocument();
  });

  it('renders content when open', () => {
    render(
      <SlidePanel isOpen={true} onClose={() => undefined} title="Test">
        <p>Content</p>
      </SlidePanel>,
    );
    expect(screen.getByText('Content')).toBeInTheDocument();
  });

  it('renders the title', () => {
    render(
      <SlidePanel isOpen={true} onClose={() => undefined} title="My Panel">
        <p>x</p>
      </SlidePanel>,
    );
    expect(screen.getByText('My Panel')).toBeInTheDocument();
  });

  it('calls onClose when overlay is clicked', () => {
    const onClose = vi.fn();
    render(
      <SlidePanel isOpen={true} onClose={onClose} title="T">
        <p>x</p>
      </SlidePanel>,
    );
    fireEvent.click(screen.getByTestId('slide-panel-overlay'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when Escape key is pressed', () => {
    const onClose = vi.fn();
    render(
      <SlidePanel isOpen={true} onClose={onClose} title="T">
        <p>x</p>
      </SlidePanel>,
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('has aria-modal attribute', () => {
    render(
      <SlidePanel isOpen={true} onClose={() => undefined} title="T">
        <p>x</p>
      </SlidePanel>,
    );
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-modal', 'true');
  });

  it('has aria-labelledby pointing to title', () => {
    render(
      <SlidePanel isOpen={true} onClose={() => undefined} title="T">
        <p>x</p>
      </SlidePanel>,
    );
    const dialog = screen.getByRole('dialog');
    const labelId = dialog.getAttribute('aria-labelledby');
    expect(labelId).toBeTruthy();
    if (!labelId) throw new Error('aria-labelledby not set');
    expect(document.getElementById(labelId)).toHaveTextContent('T');
  });
});
