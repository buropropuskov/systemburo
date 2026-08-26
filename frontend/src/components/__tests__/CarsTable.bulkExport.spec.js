import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';

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

// Лёгкий фейк ExcelJS (эталон - UserLoginHistory.spec.js): экспорт не должен тянуть
// реальную сборку книги в jsdom. Спай на addRow, чтобы проверить, какие строки
// реально попали в книгу (только выделенные, #1194 S6).
const addRowSpy = vi.hoisted(() => vi.fn());
vi.mock('exceljs', () => {
  const cell = { fill: {}, font: {}, alignment: {}, border: {} };
  const row = { height: 0, eachCell: (fn) => fn(cell), getCell: () => cell };
  const worksheet = {
    addRow: (...args) => { addRowSpy(...args); return row; },
    getCell: () => cell,
    columns: [],
  };
  const workbook = {
    addWorksheet: () => worksheet,
    xlsx: { writeBuffer: vi.fn().mockResolvedValue(new ArrayBuffer(8)) },
  };
  // Именованная function-реализация (не arrow) - vitest 4 требует конструируемую
  // функцию для `new ExcelJS.Workbook()`, иначе "is not a constructor".
  return { default: { Workbook: vi.fn(function Workbook() { return workbook; }) } };
});

import CarsTable from '../CarsTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function baseItem(overrides) {
  return {
    id: 1,
    car_number: 'А1',
    car_brand: 'BMW',
    plateNumber: 'А1',
    mark: 'BMW',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    entry_time_from: '08:00',
    entry_time_to: '18:00',
    unloadPlaces: [],
    organization_name: 'ООО',
    ...overrides,
  };
}

function mountTable() {
  return mount(CarsTable, {
    props: {
      tableName: 'kpp-1',
      tableId: 42,
      currentUserId: 1,
      currentUserName: 'Тест',
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTable - экспорт выбранных строк (#1194 S6)', () => {
  let items;

  beforeEach(() => {
    setActivePinia(createPinia());
    addRowSpy.mockReset();
    apiRequest.mockReset();
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
    global.URL.createObjectURL = vi.fn(() => 'blob:x');
    global.URL.revokeObjectURL = vi.fn();
    items = [
      baseItem({ id: 1, car_number: 'А1' }),
      baseItem({ id: 2, car_number: 'А2' }),
      baseItem({ id: 3, car_number: 'А3' }),
    ];
  });

  async function mountWithSelection(selectedRowIndexes) {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: items });
    const rows = wrapper.findAll('.item-row');
    for (const idx of selectedRowIndexes) {
      await rows[idx].trigger('click', { ctrlKey: true });
    }
    return wrapper;
  }

  it('право table.<name>.export - кнопка "Экспорт выбранных" видна ДАЖЕ без page.admin', async () => {
    usePermissionsStore().mode = 'normal';
    usePermissionsStore().effective = { 'table.kpp-1.export': { value: 'allow' } };
    const wrapper = await mountWithSelection([0, 1]);

    expect(wrapper.find('[data-testid="cars-bulk-export"]').exists()).toBe(true);
    // Перенести/Добавить/Убрать по-прежнему требуют page.admin - тут его нет.
    expect(wrapper.find('[data-testid="cars-bulk-move"]').exists()).toBe(false);
  });

  it('без права table.<name>.export кнопка "Экспорт выбранных" не рендерится', async () => {
    usePermissionsStore().mode = 'normal';
    usePermissionsStore().effective = {};
    const wrapper = await mountWithSelection([0]);

    expect(wrapper.find('[data-testid="cars-bulk-export"]').exists()).toBe(false);
  });

  it('экспортирует ТОЛЬКО выделенные строки (фильтр по selectedIds) и уведомляет об успехе', async () => {
    usePermissionsStore().mode = 'super';
    const wrapper = await mountWithSelection([0, 2]); // А1 (id 1) и А3 (id 3)
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.find('[data-testid="cars-bulk-export"]').trigger('click');
    await flushPromises();

    // Строки данных отличаются от заголовка/пустой строки-разделителя/строк
    // "Отчёт сформировал"/"Дата формирования" длиной массива (11 колонок).
    // Номер машины - третий элемент строки (см. CarsTable.buildCarsExcel).
    const dataRows = addRowSpy.mock.calls
      .slice(1)
      .filter((call) => Array.isArray(call[0]) && call[0].length === 11);
    const carNumbers = dataRows.map((call) => call[0][2]);
    expect(carNumbers).toEqual(['А1', 'А3']);
    expect(carNumbers).not.toContain('А2');

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Выгружено строк: ', bold: '2' }));
  });

  it('пустое выделение (нет совпадений в displayItems) не строит книгу', async () => {
    usePermissionsStore().mode = 'super';
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: items, selectedIds: [999] });

    await wrapper.vm.exportSelectedToExcel();
    await flushPromises();

    expect(addRowSpy).not.toHaveBeenCalled();
  });
});
