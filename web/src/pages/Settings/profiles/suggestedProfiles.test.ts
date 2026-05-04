import { describe, expect, it } from 'vitest';

import { SUGGESTED_PROFILES } from './suggestedProfiles';

describe('suggestedProfiles', () => {
  it('gives the built-in input responder comment and provide-input permissions', () => {
    const inputResponder = SUGGESTED_PROFILES.find((profile) => profile.id === 'input-responder');

    expect(inputResponder).toBeDefined();
    expect(inputResponder?.allowedActions).toEqual(['comment', 'provide_input']);
  });

  it('tells the built-in input responder to mirror its answer onto the issue', () => {
    const inputResponder = SUGGESTED_PROFILES.find((profile) => profile.id === 'input-responder');

    expect(inputResponder).toBeDefined();
    expect(inputResponder?.prompt).toContain(
      'Comment the same concise answer on the current issue',
    );
  });
});
