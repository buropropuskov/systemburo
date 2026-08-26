import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
// mounted() поднимает real-time подписку (#840); без мока реальный eventStream
// ушёл бы в fetchTicket -> reconnect с фоновым таймером на весь прогон.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import CarsTable from '../CarsTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function mountTable(props = {}) {
  return mount(CarsTable, {
    props: {
      tableName: 'КПП 1',
      tableId: 42,
      currentUserId: 1,
      currentUserName: 'Тест',
      ...props,
    },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false },
    },
  });
}

describe('CarsTable - Увеличенный режим', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    // Минимальные ответы, чтобы _loadData не падал.
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/unload-places')) return okResponse([]);
      if (url.startsWith('/license-plate-formats')) return okResponse([]);
      if (url.startsWith('/cars/active-for-table/')) return okResponse([]);
      if (url.startsWith('/cars/unload-places')) return okResponse([]);
      if (url.startsWith('/cars/history/current-status')) return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
  });

  it('по умолчанию режим выключен, класс .enlarged отсутствует', async () => {
    const wrapper = mountTable();
    await flushPromises();
    expect(wrapper.get('[data-testid=cars-table]').classes()).not.toContain('enlarged');
  });

  it('включение тоггла добавляет .enlarged и сохраняет в localStorage', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ enlarged: true });
    expect(wrapper.get('[data-testid=cars-table]').classes()).toContain('enlarged');
    expect(localStorage.getItem('enlarged-mode:cars:42')).toBe('1');
  });

  it('состояние восстанавливается из localStorage при монтировании', async () => {
    localStorage.setItem('enlarged-mode:cars:42', '1');
    const wrapper = mountTable();
    await flushPromises();
    expect(wrapper.vm.enlarged).toBe(true);
    expect(wrapper.get('[data-testid=cars-table]').classes()).toContain('enlarged');
  });

  it('состояние хранится отдельно по tableId', async () => {
    localStorage.setItem('enlarged-mode:cars:42', '1');
    localStorage.setItem('enlarged-mode:cars:43', '0');
    const wrapper = mountTable({ tableId: 43 });
    await flushPromises();
    expect(wrapper.vm.enlarged).toBe(false);
  });
});
