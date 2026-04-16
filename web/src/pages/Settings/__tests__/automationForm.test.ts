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
      autoResume: false,
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
      autoResume: false,
    });

    expect(result.success).toBe(false);
    if (result.success) {
      throw new Error('expected invalid input context regex to fail');
    }
    expect(result.error.issues.some((issue) => issue.path[0] === 'inputContextRegex')).toBe(true);
  });
});
