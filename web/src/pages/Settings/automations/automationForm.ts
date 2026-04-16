import { z } from 'zod';
import type { AutomationDef } from '../../../types/schemas';
import type { SuggestedAutomation } from './suggestedAutomations';

function isValidRegex(value: string): boolean {
  if (value.trim() === '') return true;
  try {
    new RegExp(value);
    return true;
  } catch {
    return false;
  }
}

export const automationFormSchema = z
  .object({
    id: z
      .string()
      .min(1, 'Automation ID is required.')
      .regex(/^\S+$/, 'Automation ID must not contain spaces.'),
    enabled: z.boolean(),
    profile: z.string().min(1, 'Profile is required.'),
    instructions: z.string(),
    triggerType: z.enum([
      'cron',
      'input_required',
      'tracker_comment_added',
      'issue_entered_state',
      'issue_moved_to_backlog',
      'run_failed',
    ]),
    triggerState: z.string(),
    cron: z.string(),
    timezone: z.string(),
    matchMode: z.enum(['all', 'any']),
    states: z.array(z.string().min(1)),
    labelsAny: z.array(z.string().min(1)),
    identifierRegex: z.string(),
    limit: z.string().refine((value) => value.trim() === '' || /^\d+$/.test(value.trim()), {
      message: 'Limit must be a non-negative integer.',
    }),
    inputContextRegex: z.string(),
    autoResume: z.boolean(),
  })
  .superRefine((values, ctx) => {
    if (values.triggerType === 'cron' && values.cron.trim() === '') {
      ctx.addIssue({
        code: 'custom',
        path: ['cron'],
        message: 'Cron automations require a cron expression.',
      });
    }
    if (values.triggerType === 'issue_entered_state' && values.triggerState.trim() === '') {
      ctx.addIssue({
        code: 'custom',
        path: ['triggerState'],
        message: 'Issue-entered-state automations require a target state.',
      });
    }
    if (!isValidRegex(values.identifierRegex)) {
      ctx.addIssue({
        code: 'custom',
        path: ['identifierRegex'],
        message: 'Identifier regex must be valid.',
      });
    }
    if (!isValidRegex(values.inputContextRegex)) {
      ctx.addIssue({
        code: 'custom',
        path: ['inputContextRegex'],
        message: 'Input-context regex must be valid.',
      });
    }
  });

export type AutomationFormValues = z.infer<typeof automationFormSchema>;

export function automationValuesFromDef(automation: AutomationDef): AutomationFormValues {
  return {
    id: automation.id,
    enabled: automation.enabled,
    profile: automation.profile,
    instructions: automation.instructions ?? '',
    triggerType: automation.trigger.type,
    triggerState: automation.trigger.state ?? '',
    cron: automation.trigger.cron ?? '',
    timezone: automation.trigger.timezone ?? '',
    matchMode: automation.filter?.matchMode ?? 'all',
    states: automation.filter?.states ?? [],
    labelsAny: automation.filter?.labelsAny ?? [],
    identifierRegex: automation.filter?.identifierRegex ?? '',
    limit:
      automation.filter?.limit !== undefined && automation.filter.limit > 0
        ? String(automation.filter.limit)
        : '',
    inputContextRegex: automation.filter?.inputContextRegex ?? '',
    autoResume: automation.policy?.autoResume ?? false,
  };
}

export function automationDefFromValues(values: AutomationFormValues): AutomationDef {
  const filter: NonNullable<AutomationDef['filter']> = {};
  const trimmedLimit = values.limit.trim();
  const parsedLimit = trimmedLimit === '' ? Number.NaN : Number.parseInt(trimmedLimit, 10);

  if (values.matchMode !== 'all') filter.matchMode = values.matchMode;
  if (values.states.length > 0) filter.states = values.states;
  if (values.labelsAny.length > 0) filter.labelsAny = values.labelsAny;
  if (values.identifierRegex.trim()) filter.identifierRegex = values.identifierRegex.trim();
  if (!Number.isNaN(parsedLimit) && parsedLimit > 0) filter.limit = parsedLimit;
  if (values.inputContextRegex.trim()) filter.inputContextRegex = values.inputContextRegex.trim();

  return {
    id: values.id.trim(),
    enabled: values.enabled,
    profile: values.profile,
    instructions: values.instructions.trim() || undefined,
    trigger: {
      type: values.triggerType,
      cron: values.triggerType === 'cron' ? values.cron.trim() : undefined,
      timezone:
        values.triggerType === 'cron' && values.timezone.trim()
          ? values.timezone.trim()
          : undefined,
      state:
        values.triggerType === 'issue_entered_state' && values.triggerState.trim()
          ? values.triggerState.trim()
          : undefined,
    },
    filter: Object.keys(filter).length > 0 ? filter : undefined,
    policy: values.autoResume ? { autoResume: true } : undefined,
  };
}

export function nextAutomationId(automations: readonly AutomationDef[]): string {
  let index = automations.length + 1;
  while (automations.some((automation) => automation.id === `automation-${String(index)}`)) {
    index += 1;
  }
  return `automation-${String(index)}`;
}

export function emptyAutomationValues(
  defaultProfile: string | undefined,
  automations: readonly AutomationDef[],
): AutomationFormValues {
  return {
    id: nextAutomationId(automations),
    enabled: true,
    profile: defaultProfile ?? '',
    instructions: '',
    triggerType: 'cron',
    triggerState: '',
    cron: '0 9 * * 1-5',
    timezone: '',
    matchMode: 'all',
    states: [],
    labelsAny: [],
    identifierRegex: '',
    limit: '',
    inputContextRegex: '',
    autoResume: false,
  };
}

export function automationValuesFromSuggestion(
  suggestion: SuggestedAutomation,
): AutomationFormValues {
  return {
    id: suggestion.id,
    enabled: true,
    profile: suggestion.profile,
    instructions: suggestion.instructions,
    triggerType: suggestion.triggerType,
    triggerState: suggestion.triggerState ?? '',
    cron: suggestion.cron ?? '',
    timezone: suggestion.timezone ?? '',
    matchMode: suggestion.matchMode ?? 'all',
    states: suggestion.states ?? [],
    labelsAny: suggestion.labelsAny ?? [],
    identifierRegex: suggestion.identifierRegex ?? '',
    limit: suggestion.limit ? String(suggestion.limit) : '',
    inputContextRegex: suggestion.inputContextRegex ?? '',
    autoResume: suggestion.autoResume ?? false,
  };
}
