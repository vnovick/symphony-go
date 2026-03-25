/**
 * TypeScript types matching the Go API.
 *
 * Types are derived from the Zod schemas in `./schemas.ts` — edit schemas.ts
 * to change a shape. This file exists purely for backward-compatible re-exports
 * so existing `import ... from '../types/symphony'` imports keep working.
 *
 * @deprecated Import directly from './schemas' instead. This barrel will be
 * removed in a future cleanup pass.
 */
export type {
  CommentRow,
  RunningRow,
  HistoryRow,
  RetryRow,
  Counts,
  RateLimitInfo,
  ProfileDef,
  StateSnapshot,
  LogEventType,
  IssueLogEntry,
  TrackerIssue,
} from './schemas';
