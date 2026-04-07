import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ModalFooter } from '../ModalFooter';

describe('ModalFooter', () => {
  it('renders children', () => {
    render(
      <ModalFooter>
        <button>Save</button>
      </ModalFooter>,
    );
    expect(screen.getByText('Save')).toBeInTheDocument();
  });

  it('defaults to justify-end', () => {
    const { container } = render(
      <ModalFooter>
        <button>OK</button>
      </ModalFooter>,
    );
    expect(container.firstChild).toHaveClass('justify-end');
  });

  it('applies justify-start when align=start', () => {
    const { container } = render(
      <ModalFooter align="start">
        <button>OK</button>
      </ModalFooter>,
    );
    expect(container.firstChild).toHaveClass('justify-start');
  });

  it('applies justify-center when align=center', () => {
    const { container } = render(
      <ModalFooter align="center">
        <button>OK</button>
      </ModalFooter>,
    );
    expect(container.firstChild).toHaveClass('justify-center');
  });

  it('has consistent mt-5 spacing', () => {
    const { container } = render(
      <ModalFooter>
        <button>OK</button>
      </ModalFooter>,
    );
    expect(container.firstChild).toHaveClass('mt-5');
  });
});
