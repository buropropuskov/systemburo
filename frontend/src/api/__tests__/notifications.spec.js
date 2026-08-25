import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));
import { apiRequest, apiRequestRaw } from '@/api/client';
import { getNotificationsPaginated, markAllNotificationsRead } from '../notifications';

function rawOk(body) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(body) };
}
function rawErr(body, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue(body) };
}
function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) };
}

describe('api/notifications', () => {
  beforeEach(() => vi.clearAllMocks());

  describe('getNotificationsPaginated', () => {
    it('шлёт limit/offset/filter и разворачивает data+meta (envelope, не .json() apiRequest)', async () => {
      apiRequestRaw.mockResolvedValue(rawOk({
        success: true,
        data: [{ id: 1 }, { id: 2 }],
        meta: { total: 42, unread_count: 7 },
      }));

      const result = await getNotificationsPaginated({ limit: 20, offset: 0, filter: 'unread' });

      expect(apiRequestRaw).toHaveBeenCalledWith('/notifications?limit=20&offset=0&filter=unread');
      expect(result).toEqual({ items: [{ id: 1 }, { id: 2 }], total: 42, unreadCount: 7 });
    });

    it('без filter не добавляет параметр в query', async () => {
      apiRequestRaw.mockResolvedValue(rawOk({ success: true, data: [], meta: { total: 0, unread_count: 0 } }));
      await getNotificationsPaginated({ limit: 20, offset: 20 });
      expect(apiRequestRaw).toHaveBeenCalledWith('/notifications?limit=20&offset=20');
    });

    it('бросает при !success', async () => {
      apiRequestRaw.mockResolvedValue(rawOk({ success: false, error: 'Не удалось' }));
      await expect(getNotificationsPaginated({ limit: 20, offset: 0 })).rejects.toThrow('Не удалось');
    });

    it('бросает при HTTP-ошибке', async () => {
      apiRequestRaw.mockResolvedValue(rawErr({ success: false, error: 'Сервер недоступен' }, 500));
      await expect(getNotificationsPaginated({ limit: 20, offset: 0 })).rejects.toThrow('Сервер недоступен');
    });

    it('пустой meta - total/unreadCount по умолчанию 0', async () => {
      apiRequestRaw.mockResolvedValue(rawOk({ success: true, data: [] }));
      const result = await getNotificationsPaginated({ limit: 20, offset: 0 });
      expect(result).toEqual({ items: [], total: 0, unreadCount: 0 });
    });
  });

  describe('markAllNotificationsRead', () => {
    it('PUT /notifications/read-all возвращает число затронутых', async () => {
      apiRequest.mockResolvedValue(okJson(5));
      const count = await markAllNotificationsRead();
      expect(apiRequest).toHaveBeenCalledWith('/notifications/read-all', { method: 'PUT' });
      expect(count).toBe(5);
    });

    it('бросает при ошибке', async () => {
      apiRequest.mockResolvedValue({ ok: false, status: 500, json: vi.fn().mockResolvedValue({ message: 'Сбой' }) });
      await expect(markAllNotificationsRead()).rejects.toThrow('Сбой');
    });
  });
});
