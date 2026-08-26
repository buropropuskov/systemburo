import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '@/api/client';
import EmployeeForm from '../EmployeeForm.vue';

// #1183 polish: форма собирает noticeGroups (таблица -> {free, windows, schedule}) и
// эмитит notices-change. Расписание сверяется по ПЕРЕСЕЧЕНИЮ окна пребывания срока с
// графиком прохода. 2026-07-13 = понедельник (day_of_week 0).

const TABLES = [
  {
    table: { id: 20, name: 't-people', display_name: 'ПОСТ №72', table_type: 'people', status: 'active', warning: null },
    warning_windows: [],
    time_slots: [
      { id: 1, day_of_week: 0, open_time: '10:00', close_time: '12:00', is_next_day: false, is_active: true },
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

// Пн 13:00-16:00: пребывание после закрытия (12:00) -> вне графика.
const OUTSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '13:00', time_to: '16:00' };
const INSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '10:30', time_to: '11:30' };

const mountForm = (entryPeriod) =>
  mount(EmployeeForm, { props: { fieldConfig: FIELD_CFG, entryPeriod }, attachTo: document.body });

describe('EmployeeForm - авто-проверка расписания против окна пребывания (#1183 polish)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('окно пребывания вне графика -> группа с schedule.anyClosed и режимом дня', async () => {
    const w = mountForm(OUTSIDE);
    await flushPromises();
    w.vm.selectedPassageTables = [20];
    await flushPromises();

    const groups = w.vm.noticeGroups;
    expect(groups).toHaveLength(1);
    expect(groups[0].name).toBe('ПОСТ №72');
    expect(groups[0].schedule.anyClosed).toBe(true);
    expect(groups[0].schedule.days[0].hours).toEqual(['10:00—12:00']);
    expect(groups[0].schedule.days[0].open).toBe(false);
  });

  it('окно пребывания внутри графика -> предупреждения нет', async () => {
    const w = mountForm(INSIDE);
    await flushPromises();
    w.vm.selectedPassageTables = [20];
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });

  it('без срока (entryPeriod=null) расписание не проверяется', async () => {
    const w = mountForm(null);
    await flushPromises();
    w.vm.selectedPassageTables = [20];
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });
});
