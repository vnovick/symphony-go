import { describe, expect, it } from 'vitest';
import { automationFormSchema } from '../automations/automationForm';

describe('automationFormSchema', () => {
  it('rejects invalid identifier regexes', () => {
    const result = automationFormSchema.safeParse({
      id: 'comment-watch',
      enabled: true,
      profile: 'pm',
      instructions: '',
      triggerType: 'tracker_comment_added',
      triggerState: '',
      cron: '',
      timezone: '',
      matchMode: 'all',
      states: [],
      labelsAny: [],
      identifierRegex: '[',
      limit: '',
      inputContextRegex: '',
      maxAgeMinutes: '',
      autoResume: false,
      switchToProfile: '',
      switchToBackend: '',
      cooldownMinutes: '',
    });

    expect(result.success).toBe(false);
    if (result.success) {
      throw new Error('expected invalid identifier regex to fail');
    }
    expect(result.error.issues.some((issue) => issue.path[0] === 'identifierRegex')).toBe(true);
  });

  it('rejects invalid input-context regexes', () => {
    const result = automationFormSchema.safeParse({
      id: 'input-responder',
      enabled: true,
      profile: 'pm',
      instructions: '',
      triggerType: 'input_required',
      triggerState: '',
      cron: '',
      timezone: '',
      matchMode: 'all',
      states: [],
      labelsAny: [],
      identifierRegex: '',
      limit: '',
      inputContextRegex: '[',
      maxAgeMinutes: '',
      autoResume: false,
      switchToProfile: '',
      switchToBackend: '',
      cooldownMinutes: '',
    });

    expect(result.success).toBe(false);
    if (result.success) {
      throw new Error('expected invalid input context regex to fail');
    }
    expect(result.error.issues.some((issue) => issue.path[0] === 'inputContextRegex')).toBe(true);
  });

  // Gap A — maxAgeMinutes accepts only non-negative integer strings.
  it('rejects negative or non-integer maxAgeMinutes values', () => {
    const base = {
      id: 'input-responder',
      enabled: true,
      profile: 'pm',
      instructions: '',
      triggerType: 'input_required' as const,
      triggerState: '',
      cron: '',
      timezone: '',
      matchMode: 'all' as const,
      states: [],
      labelsAny: [],
      identifierRegex: '',
      limit: '',
      inputContextRegex: '',
      autoResume: false,
      switchToProfile: '',
      switchToBackend: '' as const,
      cooldownMinutes: '',
    };
    for (const bad of ['-1', '1.5', 'soon']) {
      const result = automationFormSchema.safeParse({ ...base, maxAgeMinutes: bad });
      expect(result.success, `${bad} should fail`).toBe(false);
    }
    for (const good of ['', '0', '60', '1440']) {
      const result = automationFormSchema.safeParse({ ...base, maxAgeMinutes: good });
      expect(result.success, `${good} should pass`).toBe(true);
    }
  });
});
