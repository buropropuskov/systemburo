import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import EmployeesTableHistoryModal from '../EmployeesTableHistoryModal.vue';

// Per-table журнал (#1194 S6): GetByTable (/employees/history/table/:id) НЕ фильтрует
// action_type (урок #1085) - moved_between_tables/unbound_from_table доходят сырыми,
// без лейбла рендерились бы как action_type-строка.
describe('EmployeesTableHistoryModal - лейблы групповых операций (#1194 S6)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('moved_between_tables -> русский лейбл и dot-entry', () => {
    const wrapper = mount(EmployeesTableHistoryModal, {
      props: { tableId: 7 },
      global: { stubs: { teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'moved_between_tables' })).toBe('Перенесён между таблицами');
    expect(wrapper.vm.getActionClass('moved_between_tables')).toBe('dot-entry');
  });

  it('unbound_from_table -> русский лейбл и dot-exit', () => {
    const wrapper = mount(EmployeesTableHistoryModal, {
      props: { tableId: 7 },
      global: { stubs: { teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'unbound_from_table' })).toBe('Снят с таблицы');
    expect(wrapper.vm.getActionClass('unbound_from_table')).toBe('dot-exit');
  });

  it('added_to_table (#1085) по-прежнему на месте - не дублирую, только проверяю', () => {
    const wrapper = mount(EmployeesTableHistoryModal, {
      props: { tableId: 7 },
      global: { stubs: { teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'added_to_table' })).toBe('Добавлен в таблицу проходной');
  });
});
