import { describe, it, expect } from 'vitest';
import { toTermLine, entryStyle } from '../logFormatting';
import type { IssueLogEntry } from '../../types/schemas';

function entry(event: string, message = 'msg', tool?: string, level = 'INFO'): IssueLogEntry {
  return { level, event, message, tool, time: '12:00:00' };
}

describe('toTermLine', () => {
  it('maps text event correctly', () => {
    const line = toTermLine(entry('text', 'hello'));
    expect(line.prefix).toBe('>');
    expect(line.prefixColor).toBe('#4ade80');
    expect(line.text).toBe('hello');
    expect(line.time).toBe('12:00:00');
  });

  it('uses message as-is for action events (tool name already in message from backend)', () => {
    const line = toTermLine(entry('action', 'Write — wrote file', 'Write'));
    expect(line.prefix).toBe('$');
    expect(line.text).toBe('Write — wrote file');
  });

  it('appends detail info for action events when detail is present', () => {
    const entryWithDetail: IssueLogEntry = {
      level: 'INFO', event: 'action', message: 'Bash — sleep 10', tool: 'Bash', time: '12:00:00',
      detail: JSON.stringify({ exit_code: 0, output_size: 512 }),
    };
    const line = toTermLine(entryWithDetail);
    expect(line.text).toBe('Bash — sleep 10  ·  exit:0 · 512');
  });

  it('omits detail when status is success', () => {
    const entryWithDetail: IssueLogEntry = {
      level: 'INFO', event: 'action', message: 'Write — file.ts', tool: 'Write', time: '12:00:00',
      detail: JSON.stringify({ exit_code: 0, status: 'success' }),
    };
    const line = toTermLine(entryWithDetail);
    // status=success is omitted; exit_code=0 renders as exit:0
    expect(line.text).toBe('Write — file.ts  ·  exit:0');
  });

  it('omits tool name when absent in action events', () => {
    const line = toTermLine(entry('action', 'ran'));
    expect(line.text).toBe('ran');
  });

  it('maps subagent event', () => {
    const line = toTermLine(entry('subagent', 'spawned'));
    expect(line.prefix).toBe('↗');
    expect(line.prefixColor).toBe('#a78bfa');
  });

  it('maps pr event', () => {
    const line = toTermLine(entry('pr', 'opened'));
    expect(line.prefix).toBe('⎇');
  });

  it('maps turn event', () => {
    const line = toTermLine(entry('turn', 'turn 3'));
    expect(line.prefix).toBe('~');
  });

  it('maps warn event', () => {
    const line = toTermLine(entry('warn', 'slow'));
    expect(line.prefix).toBe('⚠');
  });

  it('maps ERROR level as error', () => {
    const line = toTermLine(entry('unknown', 'boom', undefined, 'ERROR'));
    expect(line.prefix).toBe('✗');
    expect(line.prefixColor).toBe('#ef4444');
  });

  it('uses dot prefix for unknown non-error events', () => {
    const line = toTermLine(entry('info', 'note'));
    expect(line.prefix).toBe('·');
  });
});

describe('entryStyle', () => {
  it('returns correct border and text colors for known event types', () => {
    expect(entryStyle('action').borderClass).toContain('yellow');
    expect(entryStyle('subagent').borderClass).toContain('purple');
    expect(entryStyle('text').borderClass).toContain('green');
    expect(entryStyle('pr').borderClass).toContain('emerald');
  });

  it('falls back gracefully for unknown events', () => {
    const style = entryStyle('unknown');
    expect(style.borderClass).toBeTruthy();
    expect(style.textClass).toBeTruthy();
  });
});
