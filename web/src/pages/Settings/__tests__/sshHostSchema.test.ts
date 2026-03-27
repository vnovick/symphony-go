import { describe, it, expect } from 'vitest';
import { sshHostSchema } from '../AddSSHHostModal';

describe('sshHostSchema', () => {
  it('accepts a valid hostname', () => {
    const result = sshHostSchema.safeParse({ host: 'build-server.example.com', description: '' });
    expect(result.success).toBe(true);
  });

  it('accepts host:port format', () => {
    const result = sshHostSchema.safeParse({ host: '192.168.1.10:2222', description: '' });
    expect(result.success).toBe(true);
  });

  it('accepts host with description', () => {
    const result = sshHostSchema.safeParse({ host: 'server1', description: '32 cores' });
    expect(result.success).toBe(true);
  });

  it('rejects empty host', () => {
    const result = sshHostSchema.safeParse({ host: '', description: '' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0].message).toContain('required');
    }
  });

  it('rejects host with spaces', () => {
    const result = sshHostSchema.safeParse({ host: 'bad host name', description: '' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0].message).toContain('spaces');
    }
  });

  it('rejects host starting with dash', () => {
    const result = sshHostSchema.safeParse({ host: '-malicious', description: '' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0].message).toContain('dash');
    }
  });

  it('allows empty description', () => {
    const result = sshHostSchema.safeParse({ host: 'server1', description: '' });
    expect(result.success).toBe(true);
  });
});
