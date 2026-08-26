import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';

// GetCarsHistoryByTable (/cars/history/table/:id) не фильтрует action_type (урок #1085),
// поэтому deactivate (крон CheckExpiredAttachments) и прочие действия текли в журнал
// сырым английским кодом - словаря для них не было.
describe('CarsTableHistoryModal - лейблы действий (#1085 follow-up)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  const mountModal = () => mount(CarsTableHistoryModal, {
    props: { cars: [], tableId: 42 },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });

  it('deactivate -> русский лейбл, красная точка', () => {
    const vm = mountModal().vm;
    expect(vm.getActionText({ action_type: 'deactivate' })).toBe('Автомобиль выведен из работы');
    expect(vm.getActionClass('deactivate')).toBe('dot-exit');
  });

  it('added_to_table/moved -> русские лейблы, зелёная точка', () => {
    const vm = mountModal().vm;
    expect(vm.getActionText({ action_type: 'added_to_table' })).toBe('Добавлен в таблицу проходной');
    expect(vm.getActionText({ action_type: 'moved_between_tables' })).toBe('Перенесён между таблицами');
    expect(vm.getActionClass('added_to_table')).toBe('dot-entry');
  });

  it('create/activate -> русские лейблы', () => {
    const vm = mountModal().vm;
    expect(vm.getActionText({ action_type: 'create' })).toBe('Подана заявка на автомобиль');
    expect(vm.getActionText({ action_type: 'activate' })).toBe('Автомобиль введён в работу');
  });
});
