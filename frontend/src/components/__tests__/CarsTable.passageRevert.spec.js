import { describe, it, expect, vi, beforeEach } from 'vitest';
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

import CarsTable from '../CarsTable.vue';
import { usePassageRevertStore } from '@/stores/passageRevert';

const TABLE_ID = 42;

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function baseItem(overrides) {
  return {
    id: 1,
    car_number: 'А111АА',
    car_brand: 'BMW',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    entry_checked: false,
    exit_checked: false,
    unloadPlaces: [],
    ...overrides,
  };
}

function mountTable() {
  return mount(CarsTable, {
    props: {
      tableName: 'КПП 1',
      tableId: TABLE_ID,
      currentUserId: 1,
      currentUserName: 'Тест',
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

/**
 * Кнопки прохода живут в разметке строки, и проверять их надо по разметке:
 * состояние компонента может быть каким угодно, а на экране кнопка окажется
 * недоступной (#2399 - тест на данных не увидел отсутствующий пейджер).
 */
function passButtons(wrapper) {
  return {
    entry: wrapper.find('[data-testid="ob-pass-entry"]'),
    exit: wrapper.find('[data-testid="ob-pass-exit"]'),
  };
}

describe('CarsTable - отмена ошибочной отметки (#2437)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/cars/history/current-status')) return okResponse([]);
      if (url.startsWith('/cars/active-for-table/')) return okResponse([]);
      return okResponse([]);
    });
  });

  it('пока отмена недоступна, отмеченный въезд остаётся неактивной кнопкой', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: [baseItem({ entry_checked: true, territory_status: 1 })] });

    const { entry } = passButtons(wrapper);
    expect(entry.text()).toBe('Въезд');
    expect(entry.attributes('disabled')).toBeDefined();
  });

  it('свою свежую отметку кнопка предлагает отменить и открывает окно причины', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({
      itemsData: [baseItem({
        entry_checked: true, territory_status: 1, can_revert: true, last_mark_table_id: TABLE_ID,
      })],
    });

    const { entry } = passButtons(wrapper);
    expect(entry.text()).toBe('Отменить');
    expect(entry.attributes('disabled')).toBeUndefined();

    await entry.trigger('click');

    const store = usePassageRevertStore();
    expect(store.request).toMatchObject({
      kind: 'cars', id: 1, direction: 'entry', tableId: TABLE_ID, subject: 'А111АА',
    });
    // Отмена не должна ходить в API мимо окна причины: причина обязательна.
    expect(apiRequest).not.toHaveBeenCalledWith(
      expect.stringContaining('/territory-status/revert'), expect.anything(),
    );
  });

  it('отметка чужого поста отсюда не отменяется', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({
      itemsData: [baseItem({
        entry_checked: true, territory_status: 1, can_revert: true, last_mark_table_id: TABLE_ID + 1,
      })],
    });

    const { entry } = passButtons(wrapper);
    expect(entry.text()).toBe('Въезд');
    expect(entry.attributes('disabled')).toBeDefined();
  });

  it('отменённый выезд снова доступен к отметке, а не мёртв', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({
      itemsData: [baseItem({
        entry_checked: false, exit_checked: true, territory_status: 2,
        can_revert: true, last_mark_table_id: TABLE_ID,
      })],
    });

    const { exit } = passButtons(wrapper);
    expect(exit.text()).toBe('Отменить');
    expect(exit.attributes('disabled')).toBeUndefined();
  });

  it('после своей отметки отмена доступна сразу, без ожидания опроса статусов', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: [baseItem()] });

    apiRequest.mockImplementation((url) => {
      if (url.endsWith('/territory-status')) return { ok: true, json: async () => ({}) };
      return okResponse([]);
    });

    await passButtons(wrapper).entry.trigger('click');
    await flushPromises();

    const row = wrapper.vm.itemsData[0];
    expect(row.can_revert).toBe(true);
    expect(row.last_mark_table_id).toBe(TABLE_ID);
    expect(passButtons(wrapper).entry.text()).toBe('Отменить');
  });
});
