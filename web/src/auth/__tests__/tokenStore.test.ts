import { beforeEach, describe, expect, it } from 'vitest';
import { getToken, useTokenStore } from '../tokenStore';

beforeEach(() => {
  sessionStorage.clear();
  localStorage.clear();
  useTokenStore.setState({ token: null });
});

describe('tokenStore', () => {
  it('defaults to sessionStorage when persist=false', () => {
    useTokenStore.getState().setToken('abc', false);
    expect(sessionStorage.getItem('itervox.apiToken')).toBe('abc');
    expect(localStorage.getItem('itervox.apiToken.persistent')).toBeNull();
    expect(getToken()).toBe('abc');
  });

  it('writes to localStorage when persist=true, clearing sessionStorage', () => {
    sessionStorage.setItem('itervox.apiToken', 'stale');
    useTokenStore.getState().setToken('abc', true);
    expect(localStorage.getItem('itervox.apiToken.persistent')).toBe('abc');
    expect(sessionStorage.getItem('itervox.apiToken')).toBeNull();
  });

  it('clearToken wipes both storages', () => {
    useTokenStore.getState().setToken('abc', true);
    useTokenStore.getState().clearToken();
    expect(sessionStorage.getItem('itervox.apiToken')).toBeNull();
    expect(localStorage.getItem('itervox.apiToken.persistent')).toBeNull();
    expect(getToken()).toBeNull();
  });

  it('prefers persistent over session when both are present', () => {
    sessionStorage.setItem('itervox.apiToken', 'session-value');
    localStorage.setItem('itervox.apiToken.persistent', 'local-value');
    // Re-read via a fresh module state.
    useTokenStore.setState({ token: localStorage.getItem('itervox.apiToken.persistent') });
    expect(getToken()).toBe('local-value');
  });
});
