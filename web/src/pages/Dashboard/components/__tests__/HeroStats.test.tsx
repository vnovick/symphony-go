import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { HeroStats } from '../HeroStats';
import { useItervoxStore } from '../../../../store/itervoxStore';

describe('HeroStats', () => {
  it('counts both input-required and pending-input-resume entries as blocked', () => {
    useItervoxStore.setState({
      snapshot: {
        generatedAt: new Date().toISOString(),
        counts: { running: 0, retrying: 0, paused: 0 },
        running: [],
        retrying: [],
        paused: [],
        maxConcurrentAgents: 3,
        inputRequired: [
          {
            identifier: 'ENG-1',
            sessionId: 's1',
            state: 'input_required',
            context: 'Need approval',
            queuedAt: new Date().toISOString(),
          },
          {
            identifier: 'ENG-2',
            sessionId: 's2',
            state: 'pending_input_resume',
            context: 'Waiting to resume',
            queuedAt: new Date().toISOString(),
          },
        ],
      },
    });

    render(<HeroStats />);

    expect(screen.getByText('Input Required')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
  });
});
