import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '@/api/client';
import EmployeeForm from '../EmployeeForm.vue';

// #1183 S4: при добавлении сотрудника показываем предупреждения выбранных таблиц
// прохода - свободный текст (table.warning) + окна, активные сейчас. У сотрудников
// нет мест разгрузки, только таблицы прохода. Окна "каждый день / весь день" -> детерминизм.

// Реальный /system-tables отдаёт обёрнутый SystemTableWithDetails: warning в table.warning.
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

// Поля необязательны (иначе canAddEmployee false), patent скрыт.
const FIELD_CFG = {
  last_name: { visible: true, required: false },
  first_name: { visible: true, required: false },
  position: { visible: true, required: false },
  citizenship: { visible: true, required: false },
  passport: { visible: true, required: false },
  patent: { visible: false, required: false },
  target_tables: { visible: true, required: false },
};

const warnCalls = () => notifyMock.mock.calls.map((c) => c[0]).filter((a) => a && a.type === 'warning');

describe('EmployeeForm - предупреждения мест прохода при добавлении (#1183 S4)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('баннер показывает warning таблицы и активное окно, скрывает отключённое', async () => {
    const w = mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    expect(w.find('[data-testid="person-place-warnings"]').exists()).toBe(false);

    w.vm.selectedPassageTables = [20];
    await flushPromises();

    const banner = w.find('[data-testid="person-place-warnings"]');
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain('Проход КПП-2');
    expect(banner.text()).toContain('Только по паспорту');
    expect(banner.text()).toContain('Активное окно');
    expect(banner.text()).not.toContain('Отключённое окно');
  });

  it('addEmployee шлёт notify type=warning по таблицам с предупреждением', async () => {
    const w = mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedCitizenship = { id: 1, name: 'РФ' };
    w.vm.selectedPassageTables = [20];
    await flushPromises();

    w.vm.addEmployee();
    await flushPromises();

    expect(w.emitted('employee-added')).toBeTruthy();
    const warns = warnCalls();
    expect(warns).toHaveLength(1);
    expect(warns[0].prefix).toContain('Проход КПП-2');
    expect(warns[0].bold).toContain('Только по паспорту');
    expect(warns[0].bold).toContain('Активное окно');
  });

  it('таблица без предупреждений - баннера нет и notify type=warning не шлётся', async () => {
    const w = mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedCitizenship = { id: 1, name: 'РФ' };
    w.vm.selectedPassageTables = [21];
    await flushPromises();

    expect(w.find('[data-testid="person-place-warnings"]').exists()).toBe(false);
    w.vm.addEmployee();
    await flushPromises();
    expect(warnCalls()).toHaveLength(0);
  });
});
