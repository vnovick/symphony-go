import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { AutomationsCard } from '../AutomationsCard';

describe('AutomationsCard', () => {
  it('renders suggested automation templates for the three built-in examples', () => {
    render(
      <AutomationsCard
        automations={[]}
        availableProfiles={['input-responder', 'qa', 'pm']}
        availableStates={['Backlog', 'Ready for QA']}
        availableLabels={['triage', 'qa']}
        onSave={vi.fn().mockResolvedValue(true)}
      />,
    );

    expect(screen.getByText('Input Responder')).toBeInTheDocument();
    expect(screen.getByText('QA Validation')).toBeInTheDocument();
    expect(screen.getByText('PM Backlog Review')).toBeInTheDocument();
    expect(screen.getAllByText('Use Template')).toHaveLength(3);
  });

  it('disables templates whose required profile is unavailable', async () => {
    const user = userEvent.setup();

    render(
      <AutomationsCard
        automations={[]}
        availableProfiles={['qa']}
        availableStates={['Backlog', 'Ready for QA']}
        availableLabels={['triage', 'qa']}
        onSave={vi.fn().mockResolvedValue(true)}
      />,
    );

    const inputResponder = screen.getByRole('button', { name: /Input Responder/i });
    expect(inputResponder).toBeDisabled();

    await user.click(inputResponder);

    expect(screen.queryByText(/Use "Input Responder" Template/i)).not.toBeInTheDocument();
    expect(
      screen.getByText(
        (_, element) =>
          element?.textContent === 'Create and enable the input-responder profile first.',
      ),
    ).toBeInTheDocument();
  });
});
