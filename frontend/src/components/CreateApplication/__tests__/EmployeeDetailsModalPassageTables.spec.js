import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  createPersonBlacklist: vi.fn().mockResolvedValue({}),
}));
vi.mock('exceljs', () => ({ default: {} }));

import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const stubs = { teleport: true, TableInfoModal: true, EmployeeHistoryModal: true, AddToBlacklistModal: true };

// Контекст проходной (PeopleTable) - source=peopletable, история идёт через
// /employees/:id/history (никакой фильтрации по action_type на бэке, см. employees_history_service.go).
function mountModal(employee, historyItems = [], source = 'peopletable') {
  apiRequest.mockReset();
  apiRequest.mockImplementation((url) => {
    if (String(url).includes('/history')) return Promise.resolve(okResponse(historyItems));
    return Promise.resolve(okResponse([]));
  });
  return mount(EmployeeDetailsModal, {
    props: {
      show: true,
      employee,
      source,
    },
    global: { stubs },
  });
}

describe('EmployeeDetailsModal - секция Места прохода: источник и снятые таблицы (#1227 P3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('нормализует объектную форму target_tables {id,name,source} (контекст проходной) и рисует бейдж по source', async () => {
    const wrapper = mountModal({
      id: 5,
      last_name: 'Иванов',
      first_name: 'Иван',
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

  it('плоская числовая форма target_tables -> source=null, бейдж НЕ рисуется', async () => {
    const wrapper = mountModal({ id: 5, last_name: 'Иванов', first_name: 'Иван', target_tables: [7] });
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.passageActiveTables).toEqual([
      { id: 7, name: 'Неизвестное место (ID: 7)', source: null },
    ]);
    expect(wrapper.text()).not.toContain('из заявки');
    expect(wrapper.text()).not.toContain('добавлено');
  });

  it('контекст заявки (source=application, плоские ID + история снятий): НЕТ бейджей и НЕТ зачёркнутых (#1227 fix)', async () => {
    const wrapper = mountModal(
      { id: 5, last_name: 'Иванов', first_name: 'Иван', target_tables: [10, 11] },
      [{ id: 1, action_type: 'unbound_from_table', table_id: 12, table_name: 'Таблица В' }],
      'application'
    );
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.passageActiveTables.every(t => t.source === null)).toBe(true);
    expect(wrapper.text()).not.toContain('из заявки');
    expect(wrapper.text()).not.toContain('добавлено');
    expect(wrapper.vm.passageRemovedTables).toEqual([]);
    expect(wrapper.findAll('.place-item--removed')).toHaveLength(0);
  });

  it('снятые/перенесённые таблицы из истории показываются зачёркнутыми, активная привязка перекрывает снятую', async () => {
    const wrapper = mountModal(
      { id: 5, last_name: 'Иванов', first_name: 'Иван', target_tables: [{ id: 11, name: 'Таблица Б', source: 'application' }] },
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

  it('«Места прохода не указаны» - только когда нет ни активных, ни снятых таблиц', async () => {
    const wrapper = mountModal({ id: 5, last_name: 'Иванов', first_name: 'Иван', target_tables: [] }, []);
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain('Места прохода не указаны');
  });
});

describe('EmployeeDetailsModal - подсветка открытой таблицы (#1050)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  // selectedTable - обёртка {table, time_slots, photos, current_status}, поэтому
  // сравнение по selectedTable.id давало undefined и класс active не появлялся ни на
  // одной строке: человек открывал таблицу и не видел, какая именно открыта. У машин
  // (VehicleDetailsModal) то же место сравнивает по selectedTable.table.id.
  it('открытая таблица помечается классом active, остальные нет', async () => {
    const wrapper = mountModal({
      id: 5,
      last_name: 'Иванов',
      first_name: 'Иван',
      target_tables: [
        { id: 10, name: 'Таблица А', source: 'manual' },
        { id: 11, name: 'Таблица Б', source: 'manual' },
      ],
    });
    await flushPromises();

    // Так модалка кладёт выбранную таблицу в showTableDetails: не голый объект, а обёртка.
    wrapper.vm.selectedTable = {
      table: { id: 11, name: 'Таблица Б' },
      time_slots: [],
      photos: [],
      current_status: 'closed',
    };
    wrapper.vm.showPlaceModal = true;
    await wrapper.vm.$nextTick();

    const items = wrapper.findAll('.place-item');
    expect(items).toHaveLength(2);
    expect(items[0].classes()).not.toContain('active');
    expect(items[1].classes(), 'подсвечена должна быть именно открытая таблица').toContain('active');
  });

  it('без открытой таблицы подсветки нет ни у одной строки', async () => {
    const wrapper = mountModal({
      id: 5,
      last_name: 'Иванов',
      first_name: 'Иван',
      target_tables: [{ id: 10, name: 'Таблица А', source: 'manual' }],
    });
    await flushPromises();

    expect(wrapper.findAll('.place-item').filter((i) => i.classes().includes('active'))).toHaveLength(0);
  });
});
