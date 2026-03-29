import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../auth';

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }));
  return `${header}.${body}.fake-signature`;
}

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  describe('setTokens', () => {
    it('stores tokens in state and localStorage', () => {
      const store = useAuthStore();
      store.setTokens('access-token', 'refresh-token');

      expect(store.token).toBe('access-token');
      expect(store.refreshToken).toBe('refresh-token');
      expect(localStorage.getItem('token')).toBe('access-token');
      expect(localStorage.getItem('refreshToken')).toBe('refresh-token');
    });
  });

  describe('clearTokens', () => {
    it('removes tokens from state and localStorage', () => {
      const store = useAuthStore();
      store.setTokens('access-token', 'refresh-token');
      store.clearTokens();

      expect(store.token).toBeNull();
      expect(store.refreshToken).toBeNull();
      expect(localStorage.getItem('token')).toBeNull();
      expect(localStorage.getItem('refreshToken')).toBeNull();
    });
  });

  describe('isAuthenticated', () => {
    it('returns true for a valid non-expired token', () => {
      const store = useAuthStore();
      const token = createMockJWT({ username: 'admin' }, 3600);
      store.setTokens(token, 'refresh');

      expect(store.isAuthenticated).toBe(true);
    });

    it('returns false for an expired token', () => {
      const store = useAuthStore();
      const token = createMockJWT({ username: 'admin' }, -100);
      store.setTokens(token, 'refresh');

      expect(store.isAuthenticated).toBe(false);
    });

    it('returns false when no token is set', () => {
      const store = useAuthStore();
      expect(store.isAuthenticated).toBe(false);
    });
  });

  describe('userType', () => {
    it('extracts type_id from token payload', () => {
      const store = useAuthStore();
      const token = createMockJWT({ type_id: 3 });
      store.setTokens(token, 'refresh');

      expect(store.userType).toBe(3);
    });

    it('returns null when no token is set', () => {
      const store = useAuthStore();
      expect(store.userType).toBeNull();
    });
  });

  describe('isAdmin', () => {
    it('returns true when type_id is 6', () => {
      const store = useAuthStore();
      const token = createMockJWT({ type_id: 6 });
      store.setTokens(token, 'refresh');

      expect(store.isAdmin).toBe(true);
    });

    it('returns false for non-admin type', () => {
      const store = useAuthStore();
      const token = createMockJWT({ type_id: 2 });
      store.setTokens(token, 'refresh');

      expect(store.isAdmin).toBe(false);
    });
  });

  describe('username', () => {
    it('extracts username from token payload', () => {
      const store = useAuthStore();
      const token = createMockJWT({ username: 'testuser' });
      store.setTokens(token, 'refresh');

      expect(store.username).toBe('testuser');
    });
  });
});
