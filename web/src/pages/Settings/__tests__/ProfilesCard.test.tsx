import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ProfilesCard } from '../ProfilesCard';

// ProfileRow now consumes useSkillsInventory() to surface the per-profile
// token-cost line. Stub it to avoid wrapping these tests with a QueryClient.
vi.mock('../../../queries/skills', () => ({
  useSkillsInventory: () => ({ data: null }),
}));

describe('ProfilesCard', () => {
  it('renders saved profiles as cards with active and inactive actions', () => {
    render(
      <ProfilesCard
        profileDefs={{
          planner: {
            command: 'claude --model claude-sonnet-4-6',
            backend: 'claude',
            prompt: 'Plan work.',
            enabled: true,
          },
          triage: {
            command: 'codex --model gpt-5.3-codex',
            backend: 'codex',
            prompt: 'Triage work.',
            enabled: false,
          },
        }}
        onUpsert={vi.fn().mockResolvedValue(true)}
        onDelete={vi.fn().mockResolvedValue(true)}
      />,
    );

    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Inactive')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deactivate' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Activate' })).toBeInTheDocument();
  });

  it('blocks renaming a profile to an existing profile name before save', async () => {
    const user = userEvent.setup();
    const onUpsert = vi.fn().mockResolvedValue(true);
    render(
      <ProfilesCard
        profileDefs={{
          qa: {
            command: 'claude --model claude-sonnet-4-6',
            backend: 'claude',
            prompt: 'QA work.',
            enabled: true,
          },
          pm: {
            command: 'codex --model gpt-5.3-codex',
            backend: 'codex',
            prompt: 'PM work.',
            enabled: true,
          },
        }}
        onUpsert={onUpsert}
        onDelete={vi.fn().mockResolvedValue(true)}
      />,
    );

    const qaCard = screen.getByText('qa').closest('article');
    expect(qaCard).not.toBeNull();
    await user.click(within(qaCard as HTMLElement).getByRole('button', { name: 'Edit' }));

    const nameInput = screen.getByLabelText('Profile Name');
    await user.clear(nameInput);
    await user.type(nameInput, 'pm');
    await user.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(onUpsert).not.toHaveBeenCalled();
    expect(screen.getByText('Profile names must be unique.')).toBeInTheDocument();
  });
});
