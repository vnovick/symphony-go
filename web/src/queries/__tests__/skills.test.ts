import { describe, expect, it } from 'vitest';
import {
  InventoryIssueSchema,
  InventorySchema,
  SkillSchema,
  type SkillsInventory,
} from '../../types/schemas';

const minimalInventory = {
  ScanTime: '2026-04-29T00:00:00Z',
  Skills: [
    {
      Name: 'demo',
      Description: 'demo skill',
      Provider: 'claude',
      Source: 'project',
      FilePath: '/p/.claude/skills/demo/SKILL.md',
      ApproxTokens: 50,
      TriggerPatterns: ['/demo'],
    },
  ],
  MCPServers: [{ Name: 'ctx7', Transport: 'stdio', Command: 'npx', Source: 'project-settings' }],
  Hooks: [],
  Instructions: [],
  Plugins: [],
  Issues: [],
};

const sampleIssue = {
  ID: 'DUPLICATE_MCP',
  Severity: 'warn',
  Title: 'Duplicate MCP server registration',
  Description: 'Two registrations match.',
  Affected: ['ctx7@project-settings', 'ctx7-dup@user-settings'],
  Fix: { Label: 'Remove duplicates', Action: 'remove-mcp', Destructive: true },
};

describe('skills schemas', () => {
  it('parses a minimal inventory', () => {
    const inv: SkillsInventory = InventorySchema.parse(minimalInventory);
    expect(inv.Skills?.[0].Name).toBe('demo');
  });

  it('skills array tolerates null/undefined', () => {
    const inv = InventorySchema.parse({
      ScanTime: '2026-04-29T00:00:00Z',
      Skills: null,
    });
    expect(inv.Skills).toBeNull();
  });

  it('Skill schema rejects missing required fields', () => {
    expect(() => SkillSchema.parse({ Description: 'no name' })).toThrow();
  });

  it('InventoryIssue schema parses with destructive Fix', () => {
    const issue = InventoryIssueSchema.parse(sampleIssue);
    expect(issue.Fix?.Destructive).toBe(true);
  });

  it('InventoryIssue schema parses without Fix', () => {
    const issue = InventoryIssueSchema.parse({
      ID: 'INFO_ONLY',
      Severity: 'info',
      Title: 'x',
      Description: 'y',
    });
    expect(issue.Fix).toBeUndefined();
  });
});
