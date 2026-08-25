import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../auth';
import { usePermissionsStore } from '../permissions';
import { tryRestoreSession } from '@/api/client';
import { stopImpersonation } from '@/api/impersonation';

vi.mock('@/api/auth', () => ({ getMe: vi.fn(async () => ({ user_type_code: 'employee' })) }));
vi.mock('@/api/permissions', () => ({
  getMyPermissions: vi.fn(async () => ({ mode: 'normal', permissions: [{ key: 'page.center', value: 'allow' }] })),
}));
vi.mock('@/api/client', () => ({ tryRestoreSession: vi.fn(async () => true) }));
vi.mock('@/api/impersonation', () => ({ stopImpersonation: vi.fn(async () => ({ ok: true })) }));

const session = {
  token: 'impersonation-token',
  expires_at: '2026-08-11T12:00:00Z',
  target: { id: 7, username: 'ivanov', full_name: 'Иванов Иван' },
};

describe('auth store - режим «войти как пользователь» (#1912)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    tryRestoreSession.mockImplementation(async () => true);
    stopImpersonation.mockImplementation(async () => ({ ok: true }));
  });

  it('вход в режим подменяет маркер и поднимает признак', async () => {
    const store = useAuthStore();
    await store.beginImpersonation(session);

    expect(store.token).toBe('impersonation-token');
    expect(store.isImpersonating).toBe(true);
    expect(store.impersonatedName).toBe('Иванов Иван');
  });

  it('вход в режим перечитывает права: интерфейс не должен остаться на чужом наборе', async () => {
    const store = useAuthStore();
    const permissions = usePermissionsStore();
    permissions.mode = 'admin';
    permissions.loaded = true;

    await store.beginImpersonation(session);

    expect(permissions.mode).toBe('normal');
    expect(permissions.hasPermission('page.center')).toBe(true);
  });

  it('возврат пишет выход в журнал и восстанавливает свою учётную запись', async () => {
    const store = useAuthStore();
    await store.beginImpersonation(session);

    const ok = await store.endImpersonation();

    expect(ok).toBe(true);
    expect(stopImpersonation).toHaveBeenCalledTimes(1);
    expect(tryRestoreSession).toHaveBeenCalledTimes(1);
    expect(store.isImpersonating).toBe(false);
  });

  it('истёкший маркер закрывает режим без запроса выхода - записывать его нечем', async () => {
    const store = useAuthStore();
    await store.beginImpersonation(session);

    await store.endImpersonation({ recordExit: false });

    expect(stopImpersonation).not.toHaveBeenCalled();
    expect(store.isImpersonating).toBe(false);
  });

  it('признак снимается до запроса выхода - иначе клиент зациклит завершение на 401', async () => {
    const store = useAuthStore();
    await store.beginImpersonation(session);

    let flagDuringRequest = null;
    stopImpersonation.mockImplementation(async () => {
      flagDuringRequest = store.isImpersonating;
      return { ok: true };
    });

    await store.endImpersonation();
    expect(flagDuringRequest).toBe(false);
  });

  it('несостоявшееся восстановление сессии сбрасывает маркер, а не оставляет чужой', async () => {
    const store = useAuthStore();
    await store.beginImpersonation(session);
    tryRestoreSession.mockImplementation(async () => false);

    const ok = await store.endImpersonation();

    expect(ok).toBe(false);
    expect(store.token).toBeNull();
    expect(store.isImpersonating).toBe(false);
  });

  it('вне режима возврат ничего не делает', async () => {
    const store = useAuthStore();
    expect(await store.endImpersonation()).toBe(false);
    expect(stopImpersonation).not.toHaveBeenCalled();
  });
});
