// TypeScript types matching the Go API

export interface CommentRow {
  author: string;
  body: string;
  createdAt: string; // RFC3339 or ""
}

export interface RunningRow {
  identifier: string;
  state: string;
  turnCount: number;
  tokens: number;
  inputTokens: number;
  outputTokens: number;
  lastEvent: string;
  lastEventAt: string | null; // RFC3339
  sessionId: string;
  workerHost: string;
  backend: string; // "claude" | "codex" | ""
  elapsedMs: number;
  startedAt: string; // RFC3339
}

export interface HistoryRow {
  identifier: string;
  title?: string;
  startedAt: string; // RFC3339
  finishedAt: string; // RFC3339
  elapsedMs: number;
  turnCount: number;
  tokens: number;
  inputTokens: number;
  outputTokens: number;
  status: 'succeeded' | 'failed' | 'cancelled';
  workerHost?: string;
  backend?: string;
  sessionId?: string;
}

export interface RetryRow {
  identifier: string;
  attempt: number;
  dueAt: string; // RFC3339
  error: string;
}

export interface Counts {
  running: number;
  retrying: number;
  paused: number;
}

export interface RateLimitInfo {
  requestsLimit: number;
  requestsRemaining: number;
  requestsReset: string | null; // RFC3339
  complexityLimit?: number;
  complexityRemaining?: number;
}

export interface ProfileDef {
  command: string;
  prompt?: string;
}

export interface StateSnapshot {
  generatedAt: string; // RFC3339
  counts: Counts;
  running: RunningRow[];
  history?: HistoryRow[];
  retrying: RetryRow[];
  paused: string[]; // identifiers of paused issues
  pausedWithPR?: Record<string, string>; // identifier → PR URL (subset of paused auto-paused due to open PR)
  maxConcurrentAgents: number;
  rateLimits: RateLimitInfo | null;
  trackerKind?: string; // "linear" | "github"
  activeProjectFilter?: string[];
  availableProfiles?: string[]; // named agent profiles defined in WORKFLOW.md
  profileDefs?: Record<string, ProfileDef>; // name → profile definition map
  agentMode?: string; // "" | "subagents" | "teams"
  activeStates?: string[];
  terminalStates?: string[];
  completionState?: string;
  backlogStates?: string[];
}

export type LogEventType =
  | 'text'
  | 'action'
  | 'subagent'
  | 'pr'
  | 'turn'
  | 'warn'
  | 'info'
  | 'error';

/** Structured log entry returned by GET /api/v1/issues/{id}/logs */
export interface IssueLogEntry {
  level: string;
  event: LogEventType;
  message: string;
  tool?: string;
  /** HH:MM:SS wall-clock time of the event */
  time?: string;
}

export interface TrackerIssue {
  identifier: string;
  title: string;
  state: string;
  description: string;
  url: string;
  orchestratorState: 'running' | 'retrying' | 'paused' | 'idle';
  turnCount: number;
  tokens: number;
  elapsedMs: number;
  lastMessage: string;
  error: string;
  // Enriched fields
  labels?: string[];
  priority?: number | null;
  branchName?: string | null;
  blockedBy?: string[];
  comments?: CommentRow[];
  ineligibleReason?: string;
  agentProfile?: string; // per-issue agent profile override name
}
