import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ json: async () => ({}) })),
}));

import { useDeletionsStore } from '@/stores/deletions';
import { useAuthStore } from '@/stores/auth';
import DeleteNotifications from '../DeleteNotifications.vue';

// Токен читается только геттером isAuthenticated (payload.exp), подпись не проверяется.
function validToken() {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }));
  return `header.${payload}.sig`;
}

function setup() {
  setActivePinia(createPinia());
  const loadDurations = vi.spyOn(useDeletionsStore(), 'loadDurations').mockResolvedValue(undefined);
  return { loadDurations, auth: useAuthStore() };
}

describe('DeleteNotifications', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('загрузка длительностей', () => {
    it('не дёргает настройки без авторизации', () => {
      const { loadDurations } = setup();
      mount(DeleteNotifications);
      expect(loadDurations).not.toHaveBeenCalled();
    });

    it('загружает сразу, если сессия уже живая', () => {
      const { loadDurations, auth } = setup();
      auth.token = validToken();
      mount(DeleteNotifications);
      expect(loadDurations).toHaveBeenCalledTimes(1);
    });

    // Регрессия: компонент смонтирован в App.vue один раз, а вход через форму уводит
    // на /news роутером без перемонтирования. С одним onMounted длительности из админки
    // не применялись, пока юзер не зайдёт в Машины/Люди/Корзину.
    it('загружает после входа в уже смонтированном приложении', async () => {
      const { loadDurations, auth } = setup();
      mount(DeleteNotifications);
      expect(loadDurations).not.toHaveBeenCalled();

      auth.token = validToken();
      await vi.waitFor(() => expect(loadDurations).toHaveBeenCalledTimes(1));
    });
  });

  it('объявляет стек скринридеру', () => {
    setup();
    const stack = mount(DeleteNotifications).get('.del-stack');
    expect(stack.attributes('role')).toBe('status');
    expect(stack.attributes('aria-live')).toBe('polite');
  });
});

// Оверлеи модалок это fixed inset:0 - они накрывают угол со стеком уведомлений.
// Пока стек стоял на 11000, тост об ошибке из открытой модалки (истории на 12000-13000,
// UserAccessModal, ApplicationHistory на 20000) уходил под затемнение, и пользователь
// не видел, почему сохранение не прошло. Гвард ловит новый слой поверх стека.
describe('z-index стека уведомлений', () => {
  const srcDir = resolve(__dirname, '../..');

  function collectZIndexes() {
    const found = [];
    for (const entry of readdirSync(srcDir, { recursive: true, withFileTypes: true })) {
      if (!entry.isFile() || !/\.(vue|css)$/.test(entry.name)) continue;
      const file = join(entry.parentPath ?? entry.path, entry.name);
      const text = readFileSync(file, 'utf8');
      for (const m of text.matchAll(/z-index:\s*(\d+)/g)) {
        found.push({ file, value: Number(m[1]) });
      }
    }
    return found;
  }

  it('выше любого другого слоя в приложении', () => {
    const all = collectZIndexes();
    const stack = all.filter(z => z.file.endsWith('DeleteNotifications.vue'));
    expect(stack).toHaveLength(1);

    const above = all.filter(z => !z.file.endsWith('DeleteNotifications.vue') && z.value >= stack[0].value);
    expect(above.map(z => `${z.file}: ${z.value}`)).toEqual([]);
  });
});
