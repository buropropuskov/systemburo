import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import CarHistoryModal from '../CarHistoryModal.vue';
import EmployeeHistoryModal from '../CreateApplication/EmployeeHistoryModal.vue';

// Бридж DateFilter(range) -> computed filteredHistory (#1097 S7):
// одиночный день эмитит selectedDate, период - rangeStart/rangeEnd. Границы дня 00:00-23:59.
const HISTORY = [
  { id: 1, created_at: '2026-05-04T10:00:00Z', action_type: 'entry', user_id: 1, user_name: 'A' },
  { id: 2, created_at: '2026-05-05T10:00:00Z', action_type: 'entry', user_id: 1, user_name: 'A' },
  { id: 3, created_at: '2026-05-06T10:00:00Z', action_type: 'entry', user_id: 1, user_name: 'A' },
];

function runBridgeSpecs(name, Component, props) {
  describe(`${name} - Date-бридж фильтра истории (#1097 S7)`, () => {
    beforeEach(() => setActivePinia(createPinia()));

    const mountIt = () => mount(Component, {
      props,
      global: { stubs: { LoaderSpinner: true, DateFilter: true, teleport: true } },
    });

    it('без выбранной даты - фильтр по дате не применяется', () => {
      const w = mountIt();
      w.vm.history = HISTORY;
      expect(w.vm.filteredHistory.map(i => i.id).sort()).toEqual([1, 2, 3]);
    });

    it('одиночный день (selectedDate) включает границы этого дня и исключает соседние', async () => {
      const w = mountIt();
      w.vm.history = HISTORY;
      w.vm.filterSelectedDate = new Date(2026, 4, 5); // 05.05.2026 00:00 local
      await w.vm.$nextTick();
      expect(w.vm.filteredHistory.map(i => i.id)).toEqual([2]);
    });

    it('период (rangeStart/rangeEnd) включает оба конца', async () => {
      const w = mountIt();
      w.vm.history = HISTORY;
      w.vm.filterRangeStart = new Date(2026, 4, 4, 0, 0, 0, 0);
      w.vm.filterRangeEnd = new Date(2026, 4, 5, 23, 59, 59, 999);
      await w.vm.$nextTick();
      expect(w.vm.filteredHistory.map(i => i.id).sort()).toEqual([1, 2]);
    });

    it('selectedDate имеет приоритет над остаточным range', async () => {
      const w = mountIt();
      w.vm.history = HISTORY;
      w.vm.filterRangeStart = new Date(2026, 4, 4);
      w.vm.filterRangeEnd = new Date(2026, 4, 6, 23, 59, 59, 999);
      w.vm.filterSelectedDate = new Date(2026, 4, 6);
      await w.vm.$nextTick();
      expect(w.vm.filteredHistory.map(i => i.id)).toEqual([3]);
    });
  });
}

runBridgeSpecs('CarHistoryModal', CarHistoryModal, { carId: 1 });
runBridgeSpecs('EmployeeHistoryModal', EmployeeHistoryModal, { lastName: 'Иванов', firstName: 'Иван' });
