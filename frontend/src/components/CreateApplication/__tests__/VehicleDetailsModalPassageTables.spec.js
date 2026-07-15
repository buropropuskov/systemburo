import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('@/api/blacklist', () => ({
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
  createVehicleBlacklist: vi.fn().mockResolvedValue({}),
}));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

import VehicleDetailsModal from '../VehicleDetailsModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const stubs = { teleport: true, UnloadPlaceModal: true, TableInfoModal: true, CarHistoryModal: true, AddToBlacklistModal: true };

// Контекст проходной (CarsTable) - source=carstable, showCarFeatures=true, история идёт
// через /cars/history/unified (никакой фильтрации по action_type на бэке, см. car_history_service.go).
function mountModal(vehicle, historyItems = [], source = 'carstable') {
  apiRequest.mockReset();
  apiRequest.mockImplementation((url) => {
    if (String(url).includes('/history')) return Promise.resolve(okResponse(historyItems));
    return Promise.resolve(okResponse([]));
  });
  return mount(VehicleDetailsModal, {
    props: {
      show: true,
      vehicle,
      source,
      showCarFeatures: true,
    },
    global: { stubs },
  });
}

describe('VehicleDetailsModal - секция Проезд: источник и снятые таблицы (#1227 P3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('нормализует объектную форму target_tables {id,name,source} (контекст проходной) и рисует бейдж по source', async () => {
    const wrapper = mountModal({
      id: 5,
      target_tables: [
        { id: 10, name: 'Таблица А', source: 'manual' },
        { id: 11, name: 'Таблица Б', source: 'application' },
      ],
    });
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.passageActiveTables).toEqual([
      { id: 10, name: 'Таблица А', source: 'manual' },
      { id: 11, name: 'Таблица Б', source: 'application' },
    ]);
    expect(wrapper.text()).toContain('добавлено');
    expect(wrapper.text()).toContain('из заявки');
  });

  it('плоская числовая форма target_tables -> source=null, бейдж НЕ рисуется (источник неизвестен)', async () => {
    const wrapper = mountModal({ id: 5, target_tables: [7] });
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.passageActiveTables).toEqual([
      { id: 7, name: 'Неизвестное место (ID: 7)', source: null },
    ]);
    expect(wrapper.text()).not.toContain('из заявки');
    expect(wrapper.text()).not.toContain('добавлено');
  });

  it('контекст заявки (source=application, плоские ID + история снятий): НЕТ бейджей и НЕТ зачёркнутых (#1227 fix - иначе каша с проходной)', async () => {
    const wrapper = mountModal(
      { id: 5, target_tables: [10, 11] },
      [{ id: 1, action_type: 'unbound_from_table', table_id: 12, table_name: 'Таблица В' }],
      'application'
    );
    await flushPromises();
    await wrapper.vm.$nextTick();

    // активные привязки видны как плоский список, но без бейджей источника
    expect(wrapper.vm.passageActiveTables.every(t => t.source === null)).toBe(true);
    expect(wrapper.text()).not.toContain('из заявки');
    expect(wrapper.text()).not.toContain('добавлено');
    // зачёркнутых снятых в заявке нет, хотя в истории есть unbound
    expect(wrapper.vm.passageRemovedTables).toEqual([]);
    expect(wrapper.findAll('.place-item--removed')).toHaveLength(0);
  });

  it('снятые/перенесённые таблицы из истории показываются зачёркнутыми, активная привязка перекрывает снятую', async () => {
    const wrapper = mountModal(
      { id: 5, target_tables: [{ id: 11, name: 'Таблица Б', source: 'application' }] },
      [
        { id: 1, action_type: 'unbound_from_table', table_id: 12, table_name: 'Таблица В' },
        // Снова активна сейчас - не должна дублироваться зачёркнутой.
        { id: 2, action_type: 'moved_between_tables', table_id: 11, table_name: 'Таблица Б' },
        // Повторное снятие той же таблицы - дедуп по table_id.
        { id: 3, action_type: 'unbound_from_table', table_id: 12, table_name: 'Таблица В' },
        { id: 4, action_type: 'entry' },
      ]
    );
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.passageRemovedTables).toEqual([{ id: 12, name: 'Таблица В' }]);
    const removedEls = wrapper.findAll('.place-item--removed');
    expect(removedEls).toHaveLength(1);
    expect(removedEls[0].text()).toBe('Таблица В');
  });

  it('«Проезд не указан» - только когда нет ни активных, ни снятых таблиц', async () => {
    const wrapper = mountModal({ id: 5, target_tables: [] }, []);
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain('Проезд не указан');
  });
});
