import type { ProfileDef } from '../../types/symphony';

export type SupportedBackend = 'claude' | 'codex';

export interface ModelOption {
  id: string;
  label: string;
}

export interface ProfileCommandDraft {
  backend: SupportedBackend;
  model: string;
  command: string;
  prompt: string;
}

export const CLAUDE_MODELS = [
  { id: 'claude-haiku-4-5-20251001', label: 'Haiku 4.5 - Fast' },
  { id: 'claude-sonnet-4-6', label: 'Sonnet 4.6 - Balanced' },
  { id: 'claude-opus-4-6', label: 'Opus 4.6 - Powerful' },
] satisfies ModelOption[];

export const CODEX_MODELS = [
  { id: 'gpt-5.3-codex', label: 'GPT-5.3-Codex - Frontier coding' },
  { id: 'gpt-5.2-codex', label: 'GPT-5.2-Codex - Long-horizon agentic coding' },
  { id: 'gpt-5.1-codex-max', label: 'GPT-5.1-Codex Max - Deep reasoning' },
  { id: 'gpt-5.1-codex', label: 'GPT-5.1-Codex - Balanced' },
  { id: 'gpt-5.1-codex-mini', label: 'GPT-5.1-Codex Mini - Faster, cheaper' },
  { id: 'gpt-5-codex', label: 'GPT-5-Codex - Stable baseline' },
  { id: 'codex-mini-latest', label: 'codex-mini-latest - Deprecated compatibility alias' },
] satisfies ModelOption[];

export function normalizeBackend(backend: string | undefined | null): SupportedBackend {
  return backend === 'codex' ? 'codex' : 'claude';
}

export function inferBackendFromCommand(cmd: string | undefined | null): SupportedBackend | null {
  const token = executableToken(cmd);
  switch (baseName(token)) {
    case 'codex':
      return 'codex';
    case 'claude':
      return 'claude';
    default:
      return null;
  }
}

export function commandToBackend(
  cmd: string | undefined | null,
  explicitBackend?: string,
): SupportedBackend {
  if (explicitBackend) return normalizeBackend(explicitBackend);
  return inferBackendFromCommand(cmd) ?? 'claude';
}

export function modelsForBackend(backend: SupportedBackend): ModelOption[] {
  return backend === 'codex' ? CODEX_MODELS : CLAUDE_MODELS;
}

export function modelDatalistId(backend: SupportedBackend): string {
  return `${backend}-models-datalist`;
}

export function commandToModel(cmd: string | undefined | null): string {
  if (!cmd) return '';
  const match = cmd.match(/(?:^|\s)--model\s+(\S+)/);
  return match ? match[1] : '';
}

export function buildCanonicalCommand(backend: SupportedBackend, model: string): string {
  const trimmed = model.trim();
  return trimmed ? `${backend} --model ${trimmed}` : backend;
}

export function normalizeCommandForSave(
  command: string | undefined | null,
  backend: SupportedBackend,
): string {
  const trimmed = command?.trim() ?? '';
  return trimmed || buildCanonicalCommand(backend, '');
}

export function isSimpleBackendCommand(
  command: string | undefined | null,
  backend: SupportedBackend,
): boolean {
  const trimmed = command?.trim() ?? '';
  if (!trimmed) return true;
  const fields = trimmed.split(/\s+/);
  if (fields.length === 1) {
    return inferBackendFromCommand(fields[0]) === backend;
  }
  if (fields.length === 3 && fields[1] === '--model') {
    return inferBackendFromCommand(fields[0]) === backend;
  }
  return false;
}

export function applyBackendSelection(
  command: string,
  currentBackend: SupportedBackend,
  nextBackend: SupportedBackend,
): { command: string; model: string } {
  const trimmed = command.trim();
  if (!trimmed || isSimpleBackendCommand(trimmed, currentBackend)) {
    return { command: buildCanonicalCommand(nextBackend, ''), model: '' };
  }
  return { command: trimmed, model: commandToModel(trimmed) };
}

export function applyModelSelection(
  command: string,
  backend: SupportedBackend,
  model: string,
): string {
  const trimmed = command.trim();
  if (!trimmed || isSimpleBackendCommand(trimmed, backend)) {
    return buildCanonicalCommand(backend, model);
  }
  return updateModelFlag(trimmed, model);
}

export function modelLabel(backend: SupportedBackend, modelId: string): string {
  return modelsForBackend(backend).find((m) => m.id === modelId)?.label ?? modelId;
}

export function draftFromProfileDef(def: ProfileDef): ProfileCommandDraft {
  const backend = commandToBackend(def.command, def.backend);
  return {
    backend,
    model: commandToModel(def.command),
    command: normalizeCommandForSave(def.command, backend),
    prompt: def.prompt ?? '',
  };
}

function updateModelFlag(command: string, model: string): string {
  const trimmed = model.trim();
  const pattern = /(^|\s)--model\s+\S+/;
  if (trimmed === '') {
    return command.replace(pattern, '').replace(/\s+/g, ' ').trim();
  }
  if (pattern.test(command)) {
    return command.replace(pattern, `$1--model ${trimmed}`);
  }
  return `${command} --model ${trimmed}`.trim();
}

function executableToken(cmd: string | undefined | null): string {
  const fields = (cmd ?? '').trim().split(/\s+/).filter(Boolean);
  let idx = 0;
  while (idx < fields.length && isEnvAssignment(fields[idx])) {
    idx += 1;
  }
  if (idx < fields.length && baseName(fields[idx]) === 'env') {
    idx += 1;
    while (idx < fields.length) {
      const token = fields[idx];
      if (isEnvAssignment(token) || token.startsWith('-')) {
        idx += 1;
        continue;
      }
      return token;
    }
    return '';
  }
  return fields[idx] ?? '';
}

function baseName(token: string): string {
  if (!token) return '';
  return token.split(/[\\/]/).pop() ?? token;
}

function isEnvAssignment(token: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_]*=/.test(token);
}
