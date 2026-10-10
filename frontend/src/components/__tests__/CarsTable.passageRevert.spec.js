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

  it('сохраняет просроченную незакрытую строку, разрешённую сервером, несмотря на неактивный статус', async () => {
    const expired = { id: 17, car_number: 'TEST-2674', car_brand: 'Fixture', status: 0, territory_status: 1,
      entry_date_to: '2034-12-30', server_now: '2035-01-01T12:00:00Z',
      passage_state: {has_event:true,last_event_id:81,last_event_kind:'entry',open:true,entry_time_known:true,entry_at:'2034-12-30T10:00:00Z'},
      admission: { can_enter: false, can_exit: true } };
    apiRequest.mockImplementation(url => okResponse(url.startsWith('/cars/active-for-table/') ? [expired] : []));
    const wrapper = mountTable();
    await flushPromises();
    expect(wrapper.vm.itemsData).toHaveLength(1);
    expect(wrapper.vm.itemsData[0]).toMatchObject({id:17,territory_status:1,admission:{can_enter:false,can_exit:true}});
    wrapper.unmount();
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
    expect(entry.text()).toBe('Отмена');
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
    expect(exit.text()).toBe('Отмена');
    expect(exit.attributes('disabled')).toBeUndefined();
  });

  it('после своей отметки отмена доступна сразу, без ожидания опроса статусов', async () => {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: [baseItem()] });

    let marked = false;
    const projectedRow = () => ({
      ...baseItem(), status: 1, territory_status: marked ? 1 : null,
      passage_state: { has_event: marked, open: marked, last_event_id: marked ? 17 : 0, last_event_kind: marked ? 'entry' : null },
      admission: { can_enter: !marked, can_exit: marked },
    });
    apiRequest.mockImplementation((url) => {
      if (url.endsWith('/territory-status')) { marked = true; return okResponse({}); }
      if (url.startsWith('/cars/active-for-table/')) return okResponse([projectedRow()]);
      if (url.startsWith('/cars/history/current-status')) return okResponse([{
        car_id: 1, can_revert: marked, last_mark_table_id: TABLE_ID, passage_state: projectedRow().passage_state,
      }]);
      return okResponse([]);
    });

    await passButtons(wrapper).entry.trigger('click');
    await flushPromises();

    const row = wrapper.vm.itemsData[0];
    expect(row.can_revert).toBe(true);
    expect(row.last_mark_table_id).toBe(TABLE_ID);
    expect(passButtons(wrapper).entry.text()).toBe('Отмена');
  });

// На стенде подпись у кнопки менялась, а зелёная заливка «отмечено» оставалась:
// правило `.action-btn.revertable` проигрывало по специфичности уже стоящему
// `.action-btn.entry-btn.active`. Класс на кнопке проверяем здесь, сам селектор -
// в passage.css (jsdom стили из отдельного файла не применяет).
it('кнопка отмены несёт собственный класс поверх активного состояния', async () => {
  const wrapper = mountTable();
  await flushPromises();
  await wrapper.setData({
    itemsData: [baseItem({
      entry_checked: true, territory_status: 1, can_revert: true, last_mark_table_id: TABLE_ID,
    })],
  });

  const cls = passButtons(wrapper).entry.classes();
  expect(cls).toContain('active');
  expect(cls).toContain('revertable');
});
});
