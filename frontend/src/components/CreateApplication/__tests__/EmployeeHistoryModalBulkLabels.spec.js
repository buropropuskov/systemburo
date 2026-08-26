import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import EmployeeHistoryModal from '../EmployeeHistoryModal.vue';

// Групповые операции над таблицами проходной (#1194 S2/S6): moved_between_tables и
// unbound_from_table не должны рендериться сырым action_type (урок #951/#1085).
describe('EmployeeHistoryModal - лейблы групповых операций (#1194 S6)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('moved_between_tables -> русский лейбл и dot-update', () => {
    const wrapper = mount(EmployeeHistoryModal, {
      props: { lastName: 'Иванов', firstName: 'Иван' },
      global: { stubs: { teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'moved_between_tables' })).toBe('Перенесён между таблицами');
    expect(wrapper.vm.getActionClass('moved_between_tables')).toBe('dot-update');
  });

  it('unbound_from_table -> русский лейбл и dot-deactivate', () => {
    const wrapper = mount(EmployeeHistoryModal, {
      props: { lastName: 'Иванов', firstName: 'Иван' },
      global: { stubs: { teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'unbound_from_table' })).toBe('Снят с таблицы');
    expect(wrapper.vm.getActionClass('unbound_from_table')).toBe('dot-deactivate');
  });
});
