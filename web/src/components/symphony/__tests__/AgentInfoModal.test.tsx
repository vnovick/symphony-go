import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AgentInfoModal } from '../AgentInfoModal';

vi.mock('../../ui/modal', () => ({
  Modal: ({ isOpen, children }: { isOpen: boolean; children: React.ReactNode }) =>
    isOpen ? <div data-testid="modal">{children}</div> : null,
}));

describe('AgentInfoModal', () => {
  it('renders nothing when profileName is null', () => {
    render(<AgentInfoModal profileName={null} onClose={vi.fn()} />);
    expect(screen.queryByTestId('modal')).toBeNull();
  });

  it('renders profile name as heading', () => {
    render(<AgentInfoModal profileName="reviewer" onClose={vi.fn()} />);
    expect(screen.getByText('reviewer')).toBeInTheDocument();
  });

  it('shows backend badge when profileDef has backend', () => {
    render(
      <AgentInfoModal
        profileName="reviewer"
        profileDef={{ command: 'claude', backend: 'claude', prompt: '' }}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText('claude')).toBeInTheDocument();
  });

  it('shows prompt text when available', () => {
    render(
      <AgentInfoModal
        profileName="reviewer"
        profileDef={{ command: 'claude', prompt: 'You are a code reviewer.' }}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText('You are a code reviewer.')).toBeInTheDocument();
  });

  it('shows fallback message when no prompt', () => {
    render(
      <AgentInfoModal
        profileName="reviewer"
        profileDef={{ command: 'claude' }}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/No prompt configured/)).toBeInTheDocument();
  });
});
