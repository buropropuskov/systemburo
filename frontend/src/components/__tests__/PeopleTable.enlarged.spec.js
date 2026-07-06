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

import PeopleTable from '../PeopleTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function mountTable(props = {}) {
  return mount(PeopleTable, {
    props: {
      tableName: 'КПП-72',
      currentUserId: 1,
      currentUserName: 'Тест',
      ...props,
    },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false },
    },
  });
}

describe('PeopleTable - Увеличенный режим', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) {
        return okResponse({ table: { id: 7 } });
      }
      if (url.startsWith('/system-tables')) return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/employees/active-for-table/')) return okResponse([]);
      if (url.startsWith('/employees/history/current-status')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
  });

  it('по умолчанию режим выключен, класс .enlarged отсутствует', async () => {
    const wrapper = mountTable();
    await flushPromises();
    expect(wrapper.get('[data-testid=people-table]').classes()).not.toContain('enlarged');
  });

  it('включение тоггла добавляет .enlarged и сохраняет в localStorage', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ enlarged: true });
    expect(wrapper.get('[data-testid=people-table]').classes()).toContain('enlarged');
    expect(localStorage.getItem('enlarged-mode:people:КПП-72')).toBe('1');
  });

  it('состояние восстанавливается из localStorage при монтировании', async () => {
    localStorage.setItem('enlarged-mode:people:КПП-72', '1');
    const wrapper = mountTable();
    await flushPromises();
    expect(wrapper.vm.enlarged).toBe(true);
    expect(wrapper.get('[data-testid=people-table]').classes()).toContain('enlarged');
  });

  it('состояние хранится отдельно по tableName', async () => {
    localStorage.setItem('enlarged-mode:people:КПП-72', '1');
    localStorage.setItem('enlarged-mode:people:КПП-27', '0');
    const wrapper = mountTable({ tableName: 'КПП-27' });
    await flushPromises();
    expect(wrapper.vm.enlarged).toBe(false);
  });
});
