import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Modal } from '../index';

describe('Modal', () => {
  it('renders nothing when closed', () => {
    render(
      <Modal isOpen={false} onClose={vi.fn()}>
        Content
      </Modal>,
    );
    expect(screen.queryByText('Content')).toBeNull();
  });

  it('renders children when open', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()}>
        Content
      </Modal>,
    );
    expect(screen.getByText('Content')).toBeInTheDocument();
  });

  it('has role=dialog and aria-modal', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()}>
        Test
      </Modal>,
    );
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveAttribute('aria-modal', 'true');
  });

  it('shows close button by default', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()}>
        Test
      </Modal>,
    );
    expect(screen.getByLabelText('Close')).toBeInTheDocument();
  });

  it('hides close button when showCloseButton=false', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()} showCloseButton={false}>
        Test
      </Modal>,
    );
    expect(screen.queryByLabelText('Close')).toBeNull();
  });

  it('calls onClose when close button clicked', () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen={true} onClose={onClose}>
        Test
      </Modal>,
    );
    fireEvent.click(screen.getByLabelText('Close'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('calls onClose on Escape key', () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen={true} onClose={onClose}>
        Test
      </Modal>,
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('applies padded class when padded=true', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()} padded>
        <span data-testid="inner">Padded</span>
      </Modal>,
    );
    // The p-6 is on the content wrapper div that wraps children
    const inner = screen.getByTestId('inner');
    // Walk up to find the div with p-6: inner → children wrapper (p-6)
    const wrapper = inner.closest('.p-6');
    expect(wrapper).not.toBeNull();
  });

  it('does not apply padding by default', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()}>
        <span data-testid="inner">No pad</span>
      </Modal>,
    );
    const inner = screen.getByTestId('inner');
    const wrapper = inner.closest('.p-6');
    expect(wrapper).toBeNull();
  });

  it('applies custom className', () => {
    render(
      <Modal isOpen={true} onClose={vi.fn()} className="max-w-lg">
        Test
      </Modal>,
    );
    const dialog = screen.getByRole('dialog');
    expect(dialog.className).toContain('max-w-lg');
  });
});
