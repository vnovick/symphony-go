import { describe, expect, it } from 'vitest';
import {
  AutomationDefSchema,
  ConfigInvalidStatusSchema,
  HistoryRowSchema,
  ProfileDefSchema,
  RetryRowSchema,
  RunningRowSchema,
  SSHHostInfoSchema,
  StateSnapshotSchema,
} from '../../../types/schemas';
import {
  makeAutomation,
  makeConfigInvalidStatus,
  makeHistoryRow,
  makeInputRequiredRow,
  makePendingInputResumeRow,
  makeProfileDef,
  makeRetryRow,
  makeRunningRow,
  makeSSHHostInfo,
  makeSnapshot,
} from '../snapshots';

describe('snapshot fixture factories', () => {
  it('every factory returns a schema-valid value with no overrides', () => {
    expect(() => RunningRowSchema.parse(makeRunningRow())).not.toThrow();
    expect(() => HistoryRowSchema.parse(makeHistoryRow())).not.toThrow();
    expect(() => RetryRowSchema.parse(makeRetryRow())).not.toThrow();
    expect(() => ProfileDefSchema.parse(makeProfileDef())).not.toThrow();
    expect(() => AutomationDefSchema.parse(makeAutomation())).not.toThrow();
    expect(() => SSHHostInfoSchema.parse(makeSSHHostInfo())).not.toThrow();
    expect(() => ConfigInvalidStatusSchema.parse(makeConfigInvalidStatus())).not.toThrow();
    expect(() => StateSnapshotSchema.parse(makeSnapshot())).not.toThrow();
  });

  it('input-required rows have the right state values', () => {
    expect(makeInputRequiredRow().state).toBe('input_required');
    expect(makePendingInputResumeRow().state).toBe('pending_input_resume');
  });

  it('makeSnapshot derives counts from the running/retrying/paused arrays', () => {
    const snap = makeSnapshot({
      running: [makeRunningRow(), makeRunningRow({ identifier: 'X-2' })],
      retrying: [makeRetryRow()],
      paused: ['X-3', 'X-4', 'X-5'],
    });
    expect(snap.counts.running).toBe(2);
    expect(snap.counts.retrying).toBe(1);
    expect(snap.counts.paused).toBe(3);
  });

  it('explicit counts override beats derivation', () => {
    const snap = makeSnapshot({
      running: [makeRunningRow()],
      counts: { running: 99, retrying: 0, paused: 0 },
    });
    expect(snap.counts.running).toBe(99);
  });

  it('overrides deep-merge into nested objects already present on the base', () => {
    const snap = makeSnapshot({
      profileDefs: {
        default: { command: 'codex' },
      },
    });
    // command was overridden; prompt was not (came from base default)
    expect(snap.profileDefs?.default.command).toBe('codex');
    expect(snap.profileDefs?.default.prompt).toBe('Default prompt body.');
  });

  it('overriding an array replaces wholesale', () => {
    const snap = makeSnapshot({
      activeStates: ['Custom State'],
    });
    expect(snap.activeStates).toEqual(['Custom State']);
  });

  it('factories accept overrides for every documented optional field', () => {
    const row = makeRunningRow({
      identifier: 'CUSTOM-1',
      workerHost: 'custom-host',
      backend: 'remote',
      kind: 'reviewer',
      subagentCount: 2,
    });
    expect(row.identifier).toBe('CUSTOM-1');
    expect(row.workerHost).toBe('custom-host');
    expect(row.kind).toBe('reviewer');
  });

  it('makeRunningRow omits automation fields by default and surfaces them via overrides', () => {
    const manual = makeRunningRow();
    expect(manual.automationId).toBeUndefined();
    expect(manual.triggerType).toBeUndefined();
    expect(manual.commentCount).toBeUndefined();

    const automated = makeRunningRow({
      identifier: 'AUTO-1',
      kind: 'automation',
      automationId: 'pr-on-input',
      triggerType: 'input_required',
      commentCount: 2,
    });
    expect(automated.automationId).toBe('pr-on-input');
    expect(automated.triggerType).toBe('input_required');
    expect(automated.kind).toBe('automation');
    expect(automated.commentCount).toBe(2);
  });

  it('makeHistoryRow omits automation fields by default and surfaces them via overrides', () => {
    const manual = makeHistoryRow();
    expect(manual.automationId).toBeUndefined();
    expect(manual.triggerType).toBeUndefined();
    expect(manual.commentCount).toBeUndefined();

    const automated = makeHistoryRow({
      identifier: 'AUTO-9',
      kind: 'automation',
      automationId: 'cron-nightly',
      triggerType: 'cron',
      commentCount: 1,
      status: 'succeeded',
    });
    expect(automated.automationId).toBe('cron-nightly');
    expect(automated.triggerType).toBe('cron');
    expect(automated.commentCount).toBe(1);
  });
});
