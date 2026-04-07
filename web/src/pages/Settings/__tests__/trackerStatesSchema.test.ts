import { describe, it, expect } from 'vitest';
import { trackerStatesSchema } from '../TrackerStatesCard';

describe('trackerStatesSchema', () => {
  it('accepts valid states', () => {
    const result = trackerStatesSchema.safeParse({
      activeStates: ['Todo', 'In Progress'],
      terminalStates: ['Done', 'Cancelled'],
      completionState: 'In Review',
    });
    expect(result.success).toBe(true);
  });

  it('accepts empty completionState', () => {
    const result = trackerStatesSchema.safeParse({
      activeStates: ['Todo'],
      terminalStates: ['Done'],
      completionState: '',
    });
    expect(result.success).toBe(true);
  });

  it('accepts empty terminalStates', () => {
    const result = trackerStatesSchema.safeParse({
      activeStates: ['Todo'],
      terminalStates: [],
      completionState: '',
    });
    expect(result.success).toBe(true);
  });

  it('rejects empty activeStates', () => {
    const result = trackerStatesSchema.safeParse({
      activeStates: [],
      terminalStates: ['Done'],
      completionState: '',
    });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0].message).toContain('active state');
    }
  });

  it('rejects overlapping active and terminal states', () => {
    const result = trackerStatesSchema.safeParse({
      activeStates: ['Todo', 'In Progress'],
      terminalStates: ['In Progress', 'Done'],
      completionState: '',
    });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0].message).toContain('overlap');
    }
  });

  it('accepts non-overlapping states', () => {
    const result = trackerStatesSchema.safeParse({
      activeStates: ['Todo'],
      terminalStates: ['Done'],
      completionState: 'Review',
    });
    expect(result.success).toBe(true);
  });
});
