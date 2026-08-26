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

// deactivate (крон CheckExpiredAttachments) тоже течёт в журнал через GetByTable, а
// словаря для него не было - в истории показывалось сырое английское "deactivate".
describe('EmployeesTableHistoryModal - deactivate и прочие действия', () => {
  beforeEach(() => setActivePinia(createPinia()));

  const mountModal = () => mount(EmployeesTableHistoryModal, {
    props: { tableId: 7 },
    global: { stubs: { teleport: true } },
  });

  it('deactivate -> русский лейбл, не сырой код', () => {
    const vm = mountModal().vm;
    expect(vm.getActionText({ action_type: 'deactivate' })).toBe('Сотрудник выведен из работы');
    expect(vm.getActionClass('deactivate')).toBe('dot-exit');
  });

  it('create/activate -> русские лейблы, зелёная точка', () => {
    const vm = mountModal().vm;
    expect(vm.getActionText({ action_type: 'create' })).toBe('Подана заявка на сотрудника');
    expect(vm.getActionText({ action_type: 'activate' })).toBe('Сотрудник введён в работу');
    expect(vm.getActionClass('activate')).toBe('dot-entry');
  });
});

// ФИО в журнале таблицы бралось из item.last_name, а GetByTable отдаёт employee_last_name
// - поле было undefined, и заголовок всегда падал на заглушку "ID: N".
describe('EmployeesTableHistoryModal - ФИО из employee_last_name', () => {
  beforeEach(() => setActivePinia(createPinia()));

  const mountModal = () => mount(EmployeesTableHistoryModal, {
    props: { tableId: 7 },
    global: { stubs: { teleport: true } },
  });

  it('собирает ФИО из полей employee_*', () => {
    const vm = mountModal().vm;
    expect(vm.getEmployeeName({
      employee_id: 42,
      employee_last_name: 'Иванов',
      employee_first_name: 'Иван',
      employee_middle_name: 'Иванович',
    })).toBe('Иванов Иван Иванович');
  });

  it('старое поле last_name больше не используется - остаётся заглушка ID', () => {
    const vm = mountModal().vm;
    expect(vm.getEmployeeName({ employee_id: 42, last_name: 'Иванов', first_name: 'Иван' })).toBe('ID: 42');
  });

  it('без ФИО - заглушка ID', () => {
    const vm = mountModal().vm;
    expect(vm.getEmployeeName({ employee_id: 99 })).toBe('ID: 99');
  });
});
