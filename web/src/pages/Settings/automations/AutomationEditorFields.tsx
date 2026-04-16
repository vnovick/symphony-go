import { TagInput } from '../../../components/itervox/TagInput';
import { MarkdownPromptEditor } from '../profiles/MarkdownPromptEditor';
import {
  checkboxCls,
  fieldLabelCls,
  fieldSurfaceCls,
  helperTextCls,
  inputCls,
  selectCls,
} from '../formStyles';
import type { AutomationFormValues } from './automationForm';

type AutomationTriggerType = AutomationFormValues['triggerType'];

type InstructionTemplate = {
  id: string;
  label: string;
  description: string;
  instruction: string;
  triggerTypes?: AutomationTriggerType[];
};

const TRIGGER_OPTIONS: Array<{
  value: AutomationTriggerType;
  label: string;
  description: string;
}> = [
  {
    value: 'cron',
    label: 'Cron',
    description: 'Runs on a fixed schedule and dispatches matching issues in batches.',
  },
  {
    value: 'input_required',
    label: 'Input Required',
    description: 'Dispatches when a running agent blocks and asks for human input.',
  },
  {
    value: 'tracker_comment_added',
    label: 'Tracker Comment Added',
    description: 'Polls tracker comments and fires when Itervox sees a new latest comment.',
  },
  {
    value: 'issue_entered_state',
    label: 'Issue Entered State',
    description: 'Fires when an issue transitions into a specific tracker state.',
  },
  {
    value: 'issue_moved_to_backlog',
    label: 'Issue Moved To Backlog',
    description: 'Fires when an issue newly lands in one of the configured backlog states.',
  },
  {
    value: 'run_failed',
    label: 'Run Failed',
    description: 'Fires after a worker run fails permanently and Itervox stops retrying it.',
  },
];

const VARIABLE_GROUPS = [
  {
    title: 'Issue variables',
    values: [
      '{{ issue.identifier }}',
      '{{ issue.title }}',
      '{{ issue.description }}',
      '{{ issue.state }}',
      '{{ issue.labels }}',
      '{{ issue.comments }}',
    ],
  },
  {
    title: 'Trigger variables',
    values: [
      '{{ trigger.type }}',
      '{{ trigger.fired_at }}',
      '{{ trigger.automation_id }}',
      '{{ trigger.cron }}',
      '{{ trigger.timezone }}',
      '{{ trigger.trigger_state }}',
      '{{ trigger.previous_state }}',
      '{{ trigger.current_state }}',
      '{{ trigger.input_context }}',
      '{{ trigger.blocked_profile }}',
      '{{ trigger.blocked_backend }}',
      '{{ trigger.comment.body }}',
      '{{ trigger.comment.author_name }}',
      '{{ trigger.error_message }}',
      '{{ trigger.retry_attempt }}',
    ],
  },
] as const;

const INSTRUCTION_TEMPLATES: readonly InstructionTemplate[] = [
  {
    id: 'input-responder',
    label: 'Input responder',
    description: 'Low-risk unblocker answers for input-required automations.',
    triggerTypes: ['input_required'],
    instruction: `Answer only narrow, low-risk unblocker questions.

- Prefer the safest bounded assumption that keeps work moving.
- If the request is ambiguous, state the assumption explicitly.
- If the request needs real human approval, do not invent it.
- Use \`itervox action provide-input\` only when the profile is allowed to auto-resume.`,
  },
  {
    id: 'qa-validation',
    label: 'QA validation',
    description: 'Run checks, comment results, and move the issue back when validation fails.',
    triggerTypes: ['cron', 'issue_entered_state', 'tracker_comment_added'],
    instruction: `Run the QA routine for this issue.

- Validate the change against the issue description and tracker comments.
- Comment a concise pass/fail report on the issue.
- If a required check fails, move the issue back to Todo.
- If all required checks pass, explain what was validated.`,
  },
  {
    id: 'pm-backlog-review',
    label: 'PM backlog review',
    description: 'Review issue clarity, missing context, and acceptance criteria.',
    triggerTypes: ['cron', 'issue_moved_to_backlog', 'tracker_comment_added'],
    instruction: `Review the issue for missing product detail.

- Identify vague requirements, unstated assumptions, and missing acceptance criteria.
- Leave one concise comment summarising what is unclear.
- Do not invent scope that is not supported by the issue context.`,
  },
  {
    id: 'comment-triage',
    label: 'Comment triage',
    description: 'Evaluate a newly added tracker comment and decide whether it changes next steps.',
    triggerTypes: ['tracker_comment_added'],
    instruction: `Review the newly added tracker comment in context.

- Summarise whether the comment adds actionable new information.
- If the comment resolves a blocker, say what changed.
- If the comment creates new follow-up work, capture it clearly.`,
  },
  {
    id: 'failure-follow-up',
    label: 'Failure follow-up',
    description: 'Handle permanently failed runs with concise diagnosis and next-step guidance.',
    triggerTypes: ['run_failed'],
    instruction: `Review the failed run context.

- Summarise the likely failure mode using the automation trigger data and issue context.
- Comment the next best step for a human or another agent.
- If the issue should move back to backlog or Todo, do that explicitly.`,
  },
];

function triggerDescription(triggerType: AutomationTriggerType): string {
  return (
    TRIGGER_OPTIONS.find((option) => option.value === triggerType)?.description ??
    'Choose what should wake this automation up.'
  );
}

export function AutomationEditorFields({
  values,
  availableProfiles,
  availableStates,
  availableLabels,
  onEnabledChange,
  onProfileChange,
  onInstructionsChange,
  onTriggerTypeChange,
  onTriggerStateChange,
  onCronChange,
  onTimezoneChange,
  onMatchModeChange,
  onStatesChange,
  onLabelsAnyChange,
  onIdentifierRegexChange,
  onLimitChange,
  onInputContextRegexChange,
  onAutoResumeChange,
}: {
  values: AutomationFormValues;
  availableProfiles: string[];
  availableStates: string[];
  availableLabels: string[];
  onEnabledChange: (value: boolean) => void;
  onProfileChange: (value: string) => void;
  onInstructionsChange: (value: string) => void;
  onTriggerTypeChange: (value: AutomationTriggerType) => void;
  onTriggerStateChange: (value: string) => void;
  onCronChange: (value: string) => void;
  onTimezoneChange: (value: string) => void;
  onMatchModeChange: (value: AutomationFormValues['matchMode']) => void;
  onStatesChange: (value: string[]) => void;
  onLabelsAnyChange: (value: string[]) => void;
  onIdentifierRegexChange: (value: string) => void;
  onLimitChange: (value: string) => void;
  onInputContextRegexChange: (value: string) => void;
  onAutoResumeChange: (value: boolean) => void;
}) {
  const isCron = values.triggerType === 'cron';
  const isInputRequired = values.triggerType === 'input_required';
  const isIssueEnteredState = values.triggerType === 'issue_entered_state';
  const supportsBatchLimit =
    values.triggerType === 'cron' ||
    values.triggerType === 'tracker_comment_added' ||
    values.triggerType === 'issue_entered_state' ||
    values.triggerType === 'issue_moved_to_backlog';
  const visibleInstructionTemplates = INSTRUCTION_TEMPLATES.filter(
    (template) => !template.triggerTypes || template.triggerTypes.includes(values.triggerType),
  );

  return (
    <div className="space-y-4">
      <label className="border-theme-line bg-theme-bg-soft flex items-start gap-3 rounded-[var(--radius-sm)] border px-3 py-3">
        <input
          type="checkbox"
          checked={values.enabled}
          onChange={(event) => {
            onEnabledChange(event.target.checked);
          }}
          className={checkboxCls}
        />
        <span className="min-w-0">
          <span className="text-theme-text block text-sm font-medium">Enabled</span>
          <span className="text-theme-text-secondary block text-xs">
            Disabled automations stay in configuration but do not dispatch work.
          </span>
        </span>
      </label>

      <div className="grid gap-4 md:grid-cols-2">
        <div>
          <label className={fieldLabelCls}>Profile</label>
          <select
            value={values.profile}
            onChange={(event) => {
              onProfileChange(event.target.value);
            }}
            className={selectCls}
          >
            <option value="">Select profile</option>
            {availableProfiles.map((profile) => (
              <option key={profile} value={profile}>
                {profile}
              </option>
            ))}
          </select>
          <p className={helperTextCls}>
            The selected profile provides the base prompt, backend, and daemon actions.
          </p>
        </div>

        <div>
          <label className={fieldLabelCls}>Trigger</label>
          <select
            value={values.triggerType}
            onChange={(event) => {
              onTriggerTypeChange(event.target.value as AutomationTriggerType);
            }}
            className={selectCls}
          >
            {TRIGGER_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <p className={helperTextCls}>{triggerDescription(values.triggerType)}</p>
        </div>
      </div>

      {isIssueEnteredState && (
        <div>
          <label className={fieldLabelCls}>Entered State</label>
          <select
            aria-label="Entered State"
            value={values.triggerState}
            onChange={(event) => {
              onTriggerStateChange(event.target.value);
            }}
            className={selectCls}
          >
            <option value="">Select state</option>
            {availableStates.map((state) => (
              <option key={state} value={state}>
                {state}
              </option>
            ))}
          </select>
          <p className={helperTextCls}>
            Itervox compares the previous observed issue state with the newly fetched state and
            fires only when the issue enters this state.
          </p>
        </div>
      )}

      {isCron && (
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label className={fieldLabelCls}>Cron</label>
            <input
              value={values.cron}
              onChange={(event) => {
                onCronChange(event.target.value);
              }}
              placeholder="0 9 * * 1-5"
              className={`${inputCls} font-mono text-xs`}
            />
            <p className={helperTextCls}>Five-field cron: minute hour day month weekday.</p>
          </div>

          <div>
            <label className={fieldLabelCls}>Timezone</label>
            <input
              value={values.timezone}
              onChange={(event) => {
                onTimezoneChange(event.target.value);
              }}
              placeholder="UTC or Asia/Jerusalem"
              className={inputCls}
            />
            <p className={helperTextCls}>Leave blank to use the daemon timezone.</p>
          </div>
        </div>
      )}

      {isInputRequired && (
        <label className="border-theme-line bg-theme-bg-soft flex items-start gap-3 rounded-[var(--radius-sm)] border px-3 py-3">
          <input
            type="checkbox"
            checked={values.autoResume}
            onChange={(event) => {
              onAutoResumeChange(event.target.checked);
            }}
            className={checkboxCls}
          />
          <span className="min-w-0">
            <span className="text-theme-text block text-sm font-medium">Allow auto-resume</span>
            <span className="text-theme-text-secondary block text-xs">
              When enabled, the helper automation may call{' '}
              <span className="font-mono">itervox action provide-input</span> and resume the blocked
              run automatically.
            </span>
          </span>
        </label>
      )}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
        <div className={fieldSurfaceCls}>
          <div>
            <p className="text-theme-text text-[11px] font-semibold">Instruction templates</p>
            <p className="text-theme-muted mt-0.5 text-[11px]">
              Start from a reusable instruction block, then tailor it to the selected profile and
              trigger.
            </p>
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            {visibleInstructionTemplates.map((template) => (
              <button
                key={template.id}
                type="button"
                onClick={() => {
                  onInstructionsChange(template.instruction);
                }}
                className="border-theme-line bg-theme-panel hover:bg-theme-bg-soft rounded-[var(--radius-sm)] border p-3 text-left transition-colors"
              >
                <p className="text-theme-text text-xs font-medium">{template.label}</p>
                <p className="text-theme-muted mt-1 text-[11px] leading-relaxed">
                  {template.description}
                </p>
              </button>
            ))}
          </div>
        </div>

        <div className={fieldSurfaceCls}>
          <div>
            <p className="text-theme-text text-[11px] font-semibold">Prompt variables</p>
            <p className="text-theme-muted mt-0.5 text-[11px]">
              Liquid bindings available to this automation. Trigger variables depend on the selected
              trigger type.
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            {VARIABLE_GROUPS.map((group) => (
              <div key={group.title} className="space-y-1">
                <p className="text-theme-text text-[11px] font-medium">{group.title}</p>
                <div className="space-y-1 font-mono text-[11px]">
                  {group.values.map((value) => (
                    <p key={value}>{value}</p>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <MarkdownPromptEditor
        value={values.instructions}
        onChange={onInstructionsChange}
        label="Instructions"
        placeholder="Write small automation-specific instructions in Markdown. These are layered on top of the selected profile."
        helperText={
          <>
            Automation instructions are rendered with Liquid before each run. Use{' '}
            <span className="font-mono">{'{{ issue.* }}'}</span> and{' '}
            <span className="font-mono">{'{{ trigger.* }}'}</span> for runtime context.
          </>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div className={fieldSurfaceCls}>
          <div>
            <label className={fieldLabelCls}>How to combine multiple filters</label>
            <select
              value={values.matchMode}
              onChange={(event) => {
                onMatchModeChange(event.target.value as AutomationFormValues['matchMode']);
              }}
              className={selectCls}
            >
              <option value="all">Match all populated filters</option>
              <option value="any">Match any populated filter</option>
            </select>
            <p className={helperTextCls}>
              <span className="font-medium">All</span> is stricter and is usually what you want.
              <span className="mt-1 block">
                <span className="font-medium">Any</span> is useful for broad watch rules, such as
                “issues in these states or with these labels”.
              </span>
            </p>
          </div>

          <div>
            <label className={fieldLabelCls}>States</label>
            <TagInput
              chips={values.states}
              onChange={onStatesChange}
              placeholder="+ Add state"
              suggestions={availableStates}
              suggestionLabel="Suggested states from tracker config and currently visible issues"
              chipClassName="bg-[var(--accent-soft)] text-[var(--accent-strong)]"
              addButtonClassName="bg-[var(--accent-soft)] text-[var(--accent-strong)] hover:opacity-80"
            />
            <p className={helperTextCls}>
              For cron automations, leave empty to let Itervox use backlog and active states.
              <span className="mt-1 block">
                For event-based automations, this acts as an extra issue-state guard after the
                trigger fires.
              </span>
            </p>
          </div>

          <div>
            <label className={fieldLabelCls}>Labels Any</label>
            <TagInput
              chips={values.labelsAny}
              onChange={onLabelsAnyChange}
              placeholder="+ Add label"
              suggestions={availableLabels}
              suggestionLabel="Suggestions come from issues currently visible to Itervox; you can still type any tracker label."
              chipClassName="bg-[var(--bg-soft)] text-[var(--text-secondary)]"
              addButtonClassName="bg-[var(--bg-soft)] text-[var(--text-secondary)] hover:opacity-80"
            />
            <p className={helperTextCls}>
              Match issues that have at least one of these tracker labels.
            </p>
          </div>
        </div>

        <div className={fieldSurfaceCls}>
          <div>
            <p className="text-theme-text text-[11px] font-semibold">Filter guide</p>
            <p className="text-theme-muted mt-0.5 text-[11px]">
              Filters are optional. Use them to narrow an automation to the exact issues and
              contexts you trust it to handle.
            </p>
          </div>

          <div>
            <label className={fieldLabelCls}>Identifier Regex</label>
            <input
              value={values.identifierRegex}
              onChange={(event) => {
                onIdentifierRegexChange(event.target.value);
              }}
              placeholder="^ENG-"
              className={`${inputCls} font-mono text-xs`}
            />
            <p className={helperTextCls}>
              Apply a regular expression to issue identifiers like ENG-42.
            </p>
          </div>

          {supportsBatchLimit && (
            <div>
              <label className={fieldLabelCls}>Limit</label>
              <input
                value={values.limit}
                onChange={(event) => {
                  onLimitChange(event.target.value);
                }}
                inputMode="numeric"
                placeholder="Blank = no limit"
                className={inputCls}
              />
              <p className={helperTextCls}>
                Maximum number of matching issues to queue when one poll or cron tick finds several
                candidates at once.
              </p>
            </div>
          )}

          {isInputRequired && (
            <div>
              <label className={fieldLabelCls}>Input Context Regex</label>
              <input
                value={values.inputContextRegex}
                onChange={(event) => {
                  onInputContextRegexChange(event.target.value);
                }}
                placeholder="continue|branch"
                className={`${inputCls} font-mono text-xs`}
              />
              <p className={helperTextCls}>
                Match the blocked-agent question text before dispatching the helper profile.
              </p>
            </div>
          )}

          <div className="border-theme-line bg-theme-bg-soft text-theme-text-secondary rounded-[var(--radius-sm)] border px-3 py-3 text-[11px]">
            <p className="text-theme-text font-medium">Why states and labels use suggestions</p>
            <p className="mt-1">Match issues that have at least one of these tracker labels.</p>
            <p className="mt-1">
              States come from Itervox tracker settings plus issue states it is already seeing, so
              the state selectors stay aligned with your actual workflow.
            </p>
            <p className="mt-1">
              Labels are different: Itervox does not fetch a tracker-wide label catalogue today, so
              label suggestions are built from issues currently visible to the daemon. Free-form
              entry is still supported when the label you want is not in the suggestion list.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
