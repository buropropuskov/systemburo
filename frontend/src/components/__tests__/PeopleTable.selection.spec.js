import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
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

function baseItem(overrides) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    citizenship_name: 'РФ',
    organization_name: 'ООО',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    pass_time: '08:00',
    ...overrides,
  };
}

function mountTable() {
  return mount(PeopleTable, {
    props: {
      tableName: 'КПП-72',
      currentUserId: 1,
      currentUserName: 'Тест',
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('PeopleTable - ctrl/shift-выделение строк (#1194 S3)', () => {
  let items;

  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    // Минимальные ответы, чтобы _loadData не падал (как в PeopleTable.enlarged.spec.js).
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
    items = [
      baseItem({ id: 1, last_name: 'Иванов' }),
      baseItem({ id: 2, last_name: 'Петров' }),
      baseItem({ id: 3, last_name: 'Сидоров' }),
      baseItem({ id: 4, last_name: 'Кузнецов' }),
    ];
  });

  async function mountWithItems() {
    const wrapper = mountTable();
    await flushPromises();
    // Загрузка через API замокана пустой - строки для клик-теста подставляем
    // напрямую в itemsData (displayItems - чистый computed поверх него).
    await wrapper.setData({ itemsData: items });
    return wrapper;
  }

  it('обычный клик по строке открывает деталь сотрудника, не выделяет', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click');

    expect(wrapper.vm.showDetailsModal).toBe(true);
    expect(wrapper.vm.selectedIds).toHaveLength(0);
  });

  it('ctrl-клик выделяет строку и не открывает деталь', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[1].trigger('click', { ctrlKey: true });

    expect(wrapper.vm.showDetailsModal).toBe(false);
    expect(wrapper.vm.selectedIds).toEqual([2]);
    expect(rows[1].classes()).toContain('item-row--selected');
  });

  it('повторный ctrl-клик снимает выделение строки', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[1].trigger('click', { ctrlKey: true });
    await rows[1].trigger('click', { ctrlKey: true });

    expect(wrapper.vm.selectedIds).toHaveLength(0);
  });

  it('shift-клик выделяет диапазон от якоря до цели', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true }); // якорь = 1
    await rows[2].trigger('click', { shiftKey: true }); // диапазон 1..3

    expect(wrapper.vm.selectedIds.slice().sort()).toEqual([1, 2, 3]);
  });

  it('bulk-bar появляется при выделении и показывает счётчик', async () => {
    const wrapper = await mountWithItems();
    expect(wrapper.find('[data-testid="people-bulk-bar"]').exists()).toBe(false);

    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true });
    await rows[1].trigger('click', { ctrlKey: true });

    const bar = wrapper.find('[data-testid="people-bulk-bar"]');
    expect(bar.exists()).toBe(true);
    expect(bar.text()).toContain('Выбрано: 2');
  });

  it('"Снять выбор" очищает выделение и убирает bulk-bar', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true });

    await wrapper.find('[data-testid="people-bulk-clear"]').trigger('click');

    expect(wrapper.vm.selectedIds).toHaveLength(0);
    expect(wrapper.find('[data-testid="people-bulk-bar"]').exists()).toBe(false);
  });
});
