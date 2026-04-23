import { describe, it, expect, beforeEach } from 'vitest';
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
    it('хранит access token только в памяти, не в localStorage', () => {
      const store = useAuthStore();
      store.setTokens('access-token');

      expect(store.token).toBe('access-token');
      expect(localStorage.getItem('token')).toBeNull();
      expect(localStorage.getItem('refreshToken')).toBeNull();
    });
  });

  describe('clearTokens', () => {
    it('удаляет access token из state', () => {
      const store = useAuthStore();
      store.setTokens('access-token');
      store.clearTokens();

      expect(store.token).toBeNull();
    });
  });

  describe('isAuthenticated', () => {
    it('возвращает true когда access token валидный', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ username: 'admin' }, 3600));
      expect(store.isAuthenticated).toBe(true);
    });

    it('возвращает false когда access token истёк', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ username: 'admin' }, -100));
      expect(store.isAuthenticated).toBe(false);
    });

    it('возвращает false когда token не установлен', () => {
      const store = useAuthStore();
      expect(store.isAuthenticated).toBe(false);
    });
  });

  describe('userType', () => {
    it('извлекает type_id из payload', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 3 }));
      expect(store.userType).toBe(3);
    });

    it('возвращает null когда токена нет', () => {
      const store = useAuthStore();
      expect(store.userType).toBeNull();
    });
  });

  describe('isAdmin', () => {
    it('возвращает true когда type_id=6', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 6 }));
      expect(store.isAdmin).toBe(true);
    });

    it('возвращает false для не-админа', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 2 }));
      expect(store.isAdmin).toBe(false);
    });
  });

  describe('username', () => {
    it('извлекает username из payload', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ username: 'testuser' }));
      expect(store.username).toBe('testuser');
    });
  });
});
