import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '@/api/client';
import EmployeeForm from '../EmployeeForm.vue';

// #1183 S5: при добавлении сотрудника сверяем расписание (time_slots) выбранных таблиц
// прохода со сроком заявки (prop entryPeriod), предупреждаем неблокирующе, если проход
// закрыт на границе срока. 2026-07-13 = понедельник = day_of_week 0.

const TABLES = [
  {
    table: { id: 20, name: 't-people', display_name: 'Проход КПП-2', table_type: 'people', status: 'active', warning: null },
    warning_windows: [],
    time_slots: [
      { id: 1, day_of_week: 0, open_time: '09:00', close_time: '18:00', is_next_day: false, is_active: true },
    ],
    current_status: 'closed',
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

const warnCalls = () => notifyMock.mock.calls.map((c) => c[0]).filter((a) => a && a.type === 'warning');

const OUTSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '08:00', time_to: '20:00' };
const INSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '10:00', time_to: '17:00' };

const mountForm = (entryPeriod) =>
  mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG, entryPeriod }, attachTo: document.body });

describe('EmployeeForm - авто-проверка расписания против срока (#1183 S5)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('баннер предупреждает, если срок выходит за график работы прохода', async () => {
    const w = mountForm(OUTSIDE);
    await flushPromises();
    w.vm.selectedPassageTables = [20];
    await flushPromises();

    const banner = w.find('[data-testid="person-place-warnings"]');
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain('Проход КПП-2');
    expect(banner.text()).toContain('въезда');
    expect(banner.text()).toContain('выезда');
  });

  it('срок внутри графика -> баннера нет', async () => {
    const w = mountForm(INSIDE);
    await flushPromises();
    w.vm.selectedPassageTables = [20];
    await flushPromises();

    expect(w.find('[data-testid="person-place-warnings"]').exists()).toBe(false);
  });

  it('без срока (entryPeriod=null) расписание не проверяется', async () => {
    const w = mountForm(null);
    await flushPromises();
    w.vm.selectedPassageTables = [20];
    await flushPromises();

    expect(w.find('[data-testid="person-place-warnings"]').exists()).toBe(false);
  });

  it('addEmployee шлёт notify type=warning с предупреждением расписания', async () => {
    const w = mountForm(OUTSIDE);
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
    expect(warns[0].bold).toContain('графику работы');
  });
});
