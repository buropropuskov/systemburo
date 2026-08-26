import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../auth';
import { usePermissionsStore } from '../permissions';

vi.mock('@/api/auth', () => ({
  getMe: vi.fn(),
}));

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

    it('сбрасывает userTypeCode в null', () => {
      const store = useAuthStore();
      store.userTypeCode = 'security';
      store.clearTokens();

      expect(store.userTypeCode).toBeNull();
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

  describe('isSuperAdmin', () => {
    it('возвращает true когда is_super_admin=true в JWT', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }));
      expect(store.isSuperAdmin).toBe(true);
    });

    it('возвращает false если is_super_admin=false независимо от type_id', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 6, is_super_admin: false }));
      expect(store.isSuperAdmin).toBe(false);
    });

    it('возвращает false для обычного юзера', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 2 }));
      expect(store.isSuperAdmin).toBe(false);
    });

    it('isAdmin (legacy) возвращает то же что isSuperAdmin', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }));
      expect(store.isAdmin).toBe(true);
    });
  });

  describe('username', () => {
    it('извлекает username из payload', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ username: 'testuser' }));
      expect(store.username).toBe('testuser');
    });
  });

  describe('isSecurity', () => {
    it('возвращает true когда userTypeCode равен security', () => {
      const store = useAuthStore();
      store.userTypeCode = 'security';
      expect(store.isSecurity).toBe(true);
    });

    it('возвращает false для другого кода типа', () => {
      const store = useAuthStore();
      store.userTypeCode = 'admin';
      expect(store.isSecurity).toBe(false);
    });

    it('возвращает false когда userTypeCode не загружен', () => {
      const store = useAuthStore();
      expect(store.isSecurity).toBe(false);
    });
  });

  describe('canViewAccessibleAttachments', () => {
    it('возвращает true для охранника', () => {
      const store = useAuthStore();
      store.userTypeCode = 'security';
      expect(store.canViewAccessibleAttachments).toBe(true);
    });

    it('возвращает true для супер-админа', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ is_super_admin: true }));
      expect(store.canViewAccessibleAttachments).toBe(true);
    });

    it('возвращает false для обычного пользователя', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 2 }));
      store.userTypeCode = 'user';
      expect(store.canViewAccessibleAttachments).toBe(false);
    });

    it('возвращает true для обычного пользователя с грантом page.available', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 2 }));
      store.userTypeCode = 'user';
      const perms = usePermissionsStore();
      perms.mode = 'normal';
      perms.effective = { 'page.available': { value: 'allow', source: 'role' } };
      expect(store.canViewAccessibleAttachments).toBe(true);
    });

    it('остаётся false если у обычного пользователя грант другого ключа', () => {
      const store = useAuthStore();
      store.setTokens(createMockJWT({ type_id: 2 }));
      store.userTypeCode = 'user';
      const perms = usePermissionsStore();
      perms.mode = 'normal';
      perms.effective = { 'page.statistics': { value: 'allow', source: 'role' } };
      expect(store.canViewAccessibleAttachments).toBe(false);
    });
  });

  describe('loadUserTypeCode', () => {
    it('загружает и сохраняет user_type_code из getMe', async () => {
      const { getMe } = await import('@/api/auth');
      // getMe уже возвращает распарсенные данные (envelope снят в client.js),
      // мок отдаёт plain-объект, а не Response-like - иначе тест зелёный на сломанном коде.
      getMe.mockResolvedValue({ user_type_code: 'security' });

      const store = useAuthStore();
      await store.loadUserTypeCode();

      expect(store.userTypeCode).toBe('security');
    });

    it('оставляет null при ошибке сети', async () => {
      const { getMe } = await import('@/api/auth');
      getMe.mockRejectedValue(new Error('network error'));

      const store = useAuthStore();
      await store.loadUserTypeCode();

      expect(store.userTypeCode).toBeNull();
    });
  });
});
