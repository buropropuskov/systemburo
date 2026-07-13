import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import { useDeletionsStore } from '../deletions';

function mockResponse(data) {
  return { json: async () => data };
}

// Возвращает мс, после которых уведомление само подтверждается (onConfirm).
// Так косвенно проверяем применённую длительность, не завязываясь на внутренний ref.
function confirmDelayMs(store) {
  const onConfirm = vi.fn();
  store.enqueue({ onConfirm });
  let elapsed = 0;
  while (!onConfirm.mock.calls.length && elapsed < 120000) {
    vi.advanceTimersByTime(100);
    elapsed += 100;
  }
  return onConfirm.mock.calls.length ? elapsed : -1;
}

// Декремент progress на каждом тике копит ошибку округления, поэтому
// подтверждение может прийти на один тик (100мс) позже теоретического.
function expectDuration(store, ms) {
  const delay = confirmDelayMs(store);
  expect(delay).toBeGreaterThanOrEqual(ms);
  expect(delay).toBeLessThanOrEqual(ms + 200);
}

describe('deletions store', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    vi.useFakeTimers();
  });

  describe('loadDurations', () => {
    it('применяет длительности из успешного ответа', async () => {
      apiRequest.mockResolvedValue(mockResponse({ delete_duration: 15, restore_duration: 3 }));
      const store = useDeletionsStore();

      await store.loadDurations();

      expectDuration(store, 15000);
    });

    it('не латчит и пробует снова, если ответ без валидных значений (напр. 401 до авторизации)', async () => {
      // apiRequest разворачивает 401 в {message} - валидных длительностей нет.
      apiRequest.mockResolvedValueOnce(mockResponse({ message: 'Missing or invalid authorization header' }));
      const store = useDeletionsStore();

      await store.loadDurations();
      // Дефолт сохранился (10с), значение из ошибочного ответа не применилось.
      expectDuration(store, 10000);

      // Второй вызов (после логина) отдаёт валидные значения - они применяются.
      apiRequest.mockResolvedValueOnce(mockResponse({ delete_duration: 15, restore_duration: 3 }));
      await store.loadDurations();
      expect(apiRequest).toHaveBeenCalledTimes(2);
      expectDuration(store, 15000);
    });

    it('после успешной загрузки повторный вызов не делает новый запрос', async () => {
      apiRequest.mockResolvedValue(mockResponse({ delete_duration: 8, restore_duration: 4 }));
      const store = useDeletionsStore();

      await store.loadDurations();
      await store.loadDurations();

      expect(apiRequest).toHaveBeenCalledTimes(1);
    });

    it('не падает, если apiRequest бросает, и позволяет повторить', async () => {
      apiRequest.mockRejectedValueOnce(new Error('network'));
      const store = useDeletionsStore();

      await expect(store.loadDurations()).resolves.toBeUndefined();

      // Защёлка не выставлена - повтор отдаёт валидные значения.
      apiRequest.mockResolvedValueOnce(mockResponse({ delete_duration: 20, restore_duration: 5 }));
      await store.loadDurations();
      expectDuration(store, 20000);
    });
  });

  describe('setDurations', () => {
    it('сразу применяет новые длительности без запроса', () => {
      const store = useDeletionsStore();

      store.setDurations(25, 6);

      expect(apiRequest).not.toHaveBeenCalled();
      expectDuration(store, 25000);
    });

    it('латчит загрузку - после setDurations loadDurations не делает запрос', async () => {
      const store = useDeletionsStore();

      store.setDurations(12, 4);
      await store.loadDurations();

      expect(apiRequest).not.toHaveBeenCalled();
    });
  });

  describe('notify — типы и заголовки', () => {
    it('резолвит дефолтный заголовок для каждого типа', () => {
      const store = useDeletionsStore();

      store.notify({ bold: 'A', type: 'success' });
      store.notify({ bold: 'B', type: 'error' });
      store.notify({ bold: 'C', type: 'warning' });
      store.notify({ bold: 'D', type: 'info' });

      expect(store.items.map(i => i.title)).toEqual(['Успешно', 'Ошибка', 'Внимание', 'Уведомление']);
      expect(store.items.map(i => i.type)).toEqual(['success', 'error', 'warning', 'info']);
    });

    it('пустая строка убирает заголовок, явный title побеждает дефолт', () => {
      const store = useDeletionsStore();

      store.notify({ bold: 'A', type: 'warning', title: '' });
      store.notify({ bold: 'B', type: 'info', title: 'Свой заголовок' });

      expect(store.items[0].title).toBe('');
      expect(store.items[1].title).toBe('Свой заголовок');
    });
  });
});
