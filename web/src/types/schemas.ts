/**
 * Zod schemas for all shapes returned by the Symphony HTTP API.
 *
 * These are the authoritative type definitions. `symphony.ts` re-exports the
 * inferred TypeScript types for backward compatibility with existing imports.
 *
 * At every API boundary (fetch + SSE parse), call `.parse()` so a field rename
 * in the Go server throws a clear error in the browser console instead of
 * producing silent undefined values.
 */
import { z } from 'zod';

export const CommentRowSchema = z.object({
  author: z.string(),
  body: z.string(),
  createdAt: z.string().optional(), // omitempty — absent when nil
});

export const RunningRowSchema = z.object({
  identifier: z.string(),
  state: z.string(),
  turnCount: z.number(),
  tokens: z.number(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  lastEvent: z.string().optional(), // omitempty — absent before first event
  lastEventAt: z.string().optional(), // omitempty — absent before first event
  sessionId: z.string().optional(), // omitempty — absent until session starts
  workerHost: z.string().optional(), // omitempty — absent for local execution
  backend: z.string().optional(), // omitempty — absent when unknown
  elapsedMs: z.number(),
  startedAt: z.string(),
});

export const HistoryRowSchema = z.object({
  identifier: z.string(),
  title: z.string().optional(),
  startedAt: z.string(),
  finishedAt: z.string(),
  elapsedMs: z.number(),
  turnCount: z.number(),
  tokens: z.number(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  status: z.enum(['succeeded', 'failed', 'cancelled', 'stalled', 'input_required']),
  workerHost: z.string().optional(),
  backend: z.string().optional(),
  sessionId: z.string().optional(),
  appSessionId: z.string().optional(),
});

export const RetryRowSchema = z.object({
  identifier: z.string(),
  attempt: z.number(),
  dueAt: z.string(),
  error: z.string().optional(), // omitempty
});

export const CountsSchema = z.object({
  running: z.number(),
  retrying: z.number(),
  paused: z.number(),
});

export const RateLimitInfoSchema = z.object({
  requestsLimit: z.number(),
  requestsRemaining: z.number(),
  requestsReset: z.string().optional(), // omitempty *time.Time — absent when unset
  complexityLimit: z.number().optional(),
  complexityRemaining: z.number().optional(),
});

export const SSHHostInfoSchema = z.object({
  host: z.string(),
  description: z.string().optional(),
});

export const ProfileDefSchema = z.object({
  command: z.string(),
  prompt: z.string().optional(),
  backend: z.string().optional(),
});

export const StateSnapshotSchema = z.object({
  generatedAt: z.string(),
  pollIntervalMs: z.number().optional(), // omitempty — matches Go StateSnapshot.PollIntervalMs
  counts: CountsSchema,
  running: z.array(RunningRowSchema),
  history: z.array(HistoryRowSchema).optional(),
  retrying: z.array(RetryRowSchema),
  paused: z.array(z.string()),
  pausedWithPR: z.record(z.string(), z.string()).optional(),
  maxConcurrentAgents: z.number(),
  rateLimits: RateLimitInfoSchema.nullable(),
  trackerKind: z.string().optional(),
  activeProjectFilter: z.array(z.string()).optional(),
  availableProfiles: z.array(z.string()).optional(),
  profileDefs: z.record(z.string(), ProfileDefSchema).optional(),
  agentMode: z.string().optional(),
  activeStates: z.array(z.string()).optional(),
  terminalStates: z.array(z.string()).optional(),
  completionState: z.string().optional(),
  backlogStates: z.array(z.string()).optional(),
  autoClearWorkspace: z.boolean().optional(),
  currentAppSessionId: z.string().optional(),
  sshHosts: z.array(SSHHostInfoSchema).optional(),
  dispatchStrategy: z.string().optional(),
  defaultBackend: z.string().optional(),
  inlineInput: z.boolean().optional(),
  inputRequired: z.array(z.object({
    identifier: z.string(),
    sessionId: z.string(),
    context: z.string(),
    backend: z.string().optional(),
    profile: z.string().optional(),
    queuedAt: z.string(),
  })).optional(),
});

export const LogEventTypeSchema = z.enum([
  'text',
  'action',
  'subagent',
  'pr',
  'turn',
  'warn',
  'info',
  'error',
]);

export const IssueLogEntrySchema = z.object({
  level: z.string(),
  event: LogEventTypeSchema.catch('info'),
  message: z.string(),
  tool: z.string().optional(),
  time: z.string().optional(),
  detail: z.string().optional(),
  sessionId: z.string().optional(),
});

export const TrackerIssueSchema = z.object({
  identifier: z.string(),
  title: z.string(),
  state: z.string(),
  description: z.string().optional(), // omitempty — absent when ""
  url: z.string().optional(), // omitempty — absent when ""
  orchestratorState: z.enum(['idle', 'running', 'retrying', 'paused', 'input_required']),
  turnCount: z.number().optional(), // omitempty — absent when 0
  tokens: z.number().optional(), // omitempty — absent when 0
  elapsedMs: z.number().optional(), // omitempty — absent when 0
  lastMessage: z.string().optional(), // omitempty — absent when ""
  error: z.string().optional(), // omitempty — absent when ""
  labels: z.array(z.string()).optional(),
  priority: z.number().nullable().optional(),
  branchName: z.string().nullable().optional(),
  blockedBy: z.array(z.string()).optional(),
  comments: z.array(CommentRowSchema).optional(),
  ineligibleReason: z.string().optional(),
  agentProfile: z.string().optional(),
});

// Inferred TypeScript types — re-exported from symphony.ts for backward compatibility.
export type SSHHostInfo = z.infer<typeof SSHHostInfoSchema>;
export type CommentRow = z.infer<typeof CommentRowSchema>;
export type RunningRow = z.infer<typeof RunningRowSchema>;
export type HistoryRow = z.infer<typeof HistoryRowSchema>;
export type RetryRow = z.infer<typeof RetryRowSchema>;
export type Counts = z.infer<typeof CountsSchema>;
export type RateLimitInfo = z.infer<typeof RateLimitInfoSchema>;
export type ProfileDef = z.infer<typeof ProfileDefSchema>;
export type StateSnapshot = z.infer<typeof StateSnapshotSchema>;
export type LogEventType = z.infer<typeof LogEventTypeSchema>;
export type IssueLogEntry = z.infer<typeof IssueLogEntrySchema>;
export type TrackerIssue = z.infer<typeof TrackerIssueSchema>;
