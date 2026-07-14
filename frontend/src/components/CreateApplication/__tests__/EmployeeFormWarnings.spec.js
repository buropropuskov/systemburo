import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '@/api/client';
import EmployeeForm from '../EmployeeForm.vue';

// #1183: форма собирает noticeGroups (таблица прохода -> {free, windows, schedule}) для
// единой панели. У сотрудников нет мест разгрузки, только таблицы прохода. Окна
// "каждый день / весь день" -> активны в любой момент (детерминизм).

const TABLES = [
  {
    table: { id: 20, name: 't-people', display_name: 'Проход КПП-2', table_type: 'people', status: 'active', warning: 'Только по паспорту' },
    warning_windows: [
      { id: 1, day_of_week: null, time_from: null, time_to: null, is_next_day: false, message: 'Активное окно', is_active: true },
      { id: 2, day_of_week: null, time_from: null, time_to: null, is_next_day: false, message: 'Отключённое окно', is_active: false },
    ],
    time_slots: [], current_status: 'closed',
  },
  {
    table: { id: 21, name: 't2', display_name: 'Проход без нюансов', table_type: 'people', status: 'active', warning: null },
    warning_windows: [], time_slots: [], current_status: 'closed',
  },
];

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

const api = (url) => {
  if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => TABLES });
  return Promise.resolve({ ok: true, json: async () => [] });
};

vi.mock('@/api/client', () => ({ apiRequest: vi.fn((url) => api(url)) }));
vi.mock('@/api/blacklist', () => ({ checkPersonBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })) }));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
  default: { name: 'ExistingEmployeesModal', template: '<div />' },
}));

const FIELD_CFG = {
  last_name: { visible: true, required: false },
  first_name: { visible: true, required: false },
  position: { visible: true, required: false },
  citizenship: { visible: true, required: false },
  passport: { visible: true, required: false },
  patent: { visible: false, required: false },
  target_tables: { visible: true, required: false },
};

describe('EmployeeForm - предупреждения таблиц прохода в noticeGroups (#1183)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('группа несёт warning таблицы и активное окно, скрывает отключённое', async () => {
    const w = mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);

    w.vm.selectedPassageTables = [20];
    await flushPromises();

    const groups = w.vm.noticeGroups;
    expect(groups).toHaveLength(1);
    expect(groups[0].name).toBe('Проход КПП-2');
    expect(groups[0].free).toBe('Только по паспорту');
    expect(groups[0].windows).toContain('Активное окно');
    expect(groups[0].windows).not.toContain('Отключённое окно');
  });

  it('таблица без предупреждений - не попадает в noticeGroups', async () => {
    const w = mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedPassageTables = [21];
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });
});
