import type { IssueLogEntry } from '../types/schemas';

export interface TermLine {
  prefix: string;
  prefixColor: string;
  text: string;
  textColor: string;
  time?: string;
}

export function toTermLine(entry: IssueLogEntry): TermLine {
  const base = { time: entry.time };
  switch (entry.event) {
    case 'text':
      return {
        ...base,
        prefix: '>',
        prefixColor: '#4ade80',
        text: entry.message,
        textColor: '#e5e7eb',
      } as TermLine;
    case 'action': {
      let text = entry.message;
      if (entry.detail) {
        try {
          const d = JSON.parse(entry.detail) as Record<string, unknown>;
          const parts: string[] = [];
          if (d.exit_code !== undefined && d.exit_code !== null) parts.push(`exit:${String(d.exit_code)}`);
          if (d.output_size) parts.push(String(d.output_size));
          if (d.status && d.status !== 'success') parts.push(String(d.status));
          if (parts.length > 0) text = `${text}  ·  ${parts.join(' · ')}`;
        } catch {
          // ignore malformed detail JSON
        }
      }
      return {
        ...base,
        prefix: '$',
        prefixColor: '#facc15',
        text,
        textColor: '#d1d5db',
      } as TermLine;
    }
    case 'subagent':
      return {
        ...base,
        prefix: '↗',
        prefixColor: '#a78bfa',
        text: entry.message,
        textColor: '#c4b5fd',
      } as TermLine;
    case 'pr':
      return {
        ...base,
        prefix: '⎇',
        prefixColor: '#34d399',
        text: entry.message,
        textColor: '#6ee7b7',
      } as TermLine;
    case 'turn':
      return {
        ...base,
        prefix: '~',
        prefixColor: '#60a5fa',
        text: entry.message,
        textColor: '#93c5fd',
      } as TermLine;
    case 'warn':
      return {
        ...base,
        prefix: '⚠',
        prefixColor: '#f59e0b',
        text: entry.message,
        textColor: '#fbbf24',
      } as TermLine;
    default:
      if (entry.level === 'ERROR')
        return {
          ...base,
          prefix: '✗',
          prefixColor: '#ef4444',
          text: entry.message,
          textColor: '#fca5a5',
        } as TermLine;
      return {
        ...base,
        prefix: '·',
        prefixColor: '#71717a',
        text: entry.message,
        textColor: '#a1a1aa',
      } as TermLine;
  }
}

export interface EntryStyle {
  borderClass: string;
  textClass: string;
  prefixChar: string;
}

const EVENT_STYLES: Record<string, EntryStyle> = {
  text: { borderClass: 'border-green-500/30', textClass: 'text-green-300', prefixChar: '>' },
  action: { borderClass: 'border-yellow-500/30', textClass: 'text-yellow-200', prefixChar: '$' },
  subagent: { borderClass: 'border-purple-500/30', textClass: 'text-purple-300', prefixChar: '↗' },
  pr: { borderClass: 'border-emerald-500/30', textClass: 'text-emerald-300', prefixChar: '⎇' },
  turn: { borderClass: 'border-blue-500/30', textClass: 'text-blue-300', prefixChar: '~' },
  warn: { borderClass: 'border-amber-500/30', textClass: 'text-amber-300', prefixChar: '⚠' },
  error: { borderClass: 'border-red-500/30', textClass: 'text-red-300', prefixChar: '✗' },
};

const FALLBACK_STYLE: EntryStyle = {
  borderClass: 'border-gray-700',
  textClass: 'text-gray-400',
  prefixChar: '·',
};

export function entryStyle(event: string, level?: string): EntryStyle {
  if (level === 'ERROR') return EVENT_STYLES.error;
  return EVENT_STYLES[event] ?? FALLBACK_STYLE;
}
