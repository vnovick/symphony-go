export interface SuggestedAutomation {
  id: string;
  label: string;
  description: string;
  profile: string;
  triggerType:
    | 'cron'
    | 'input_required'
    | 'tracker_comment_added'
    | 'issue_entered_state'
    | 'issue_moved_to_backlog'
    | 'run_failed';
  instructions: string;
  triggerState?: string;
  cron?: string;
  timezone?: string;
  matchMode?: 'all' | 'any';
  states?: string[];
  labelsAny?: string[];
  identifierRegex?: string;
  limit?: number;
  inputContextRegex?: string;
  autoResume?: boolean;
}

export const SUGGESTED_AUTOMATIONS: readonly SuggestedAutomation[] = [
  {
    id: 'input-responder',
    label: 'Input Responder',
    description:
      'Dispatches a helper profile when a run blocks for input, using narrow trigger context and optional auto-resume.',
    profile: 'input-responder',
    triggerType: 'input_required',
    instructions: `Answer only narrow, low-risk unblocker questions.

- Prefer the safest bounded assumption that keeps work moving.
- If the blocked request is ambiguous, state the assumption explicitly.
- If the request needs real human approval, do not invent it.`,
    inputContextRegex: 'continue|branch|which file|test command',
    matchMode: 'all',
    autoResume: true,
  },
  {
    id: 'qa-validation',
    label: 'QA Validation',
    description:
      'Runs a QA profile on issues ready for verification, comments results, and pushes failures back to Todo.',
    profile: 'qa',
    triggerType: 'cron',
    cron: '0 */2 * * *',
    matchMode: 'all',
    instructions: `Run the QA routine for this issue.

- Validate the change against the issue description and comments.
- Comment a concise pass/fail report on the issue.
- If any required check fails, move the issue to Todo.`,
    states: ['Ready for QA'],
    limit: 10,
  },
  {
    id: 'pm-backlog-review',
    label: 'PM Backlog Review',
    description:
      'Reviews backlog issues for missing clarity, acceptance criteria, and scope gaps before engineering picks them up.',
    profile: 'pm',
    triggerType: 'cron',
    cron: '0 9 * * 1-5',
    matchMode: 'all',
    instructions: `Review the issue for missing product detail.

- Identify vague requirements, unstated assumptions, and missing acceptance criteria.
- Leave one concise comment summarising what is unclear.
- Do not rewrite the task or invent scope that is not supported by context.`,
    states: ['Backlog'],
    limit: 20,
  },
];
