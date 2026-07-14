import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { apiRequest } from '@/api/client';
import VehicleForm from '../VehicleForm.vue';

// #1183: форма собирает noticeGroups (место -> {free, windows, schedule}) для единой
// панели. Свободный текст (S1) + окна, активные сейчас (S4, is_active). Окна в фикстурах
// "каждый день / весь день" -> активны в любой момент (детерминизм).

const PLACES = [
  {
    id: 1, name: 'Пост №72', status: 'active',
    warning: 'Только малогабарит',
    warning_windows: [
      { id: 1, day_of_week: null, time_from: null, time_to: null, is_next_day: false, message: 'Активное окно', is_active: true },
      { id: 2, day_of_week: null, time_from: null, time_to: null, is_next_day: false, message: 'Отключённое окно', is_active: false },
    ],
  },
  { id: 2, name: 'Пост №5', status: 'active', warning: null, warning_windows: [] },
];

// Реальный /system-tables отдаёт обёрнутый SystemTableWithDetails: warning в table.warning.
const TABLES = [
  {
    table: { id: 10, name: 't-cars', display_name: 'Проезд Ворота-1', table_type: 'cars', status: 'active', warning: 'Высота до 3 м' },
    warning_windows: [], time_slots: [], current_status: 'closed',
  },
];

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

const api = (url) => {
  if (url === '/unload-places') return Promise.resolve({ ok: true, json: async () => PLACES });
  if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => TABLES });
  return Promise.resolve({ ok: true, json: async () => [] });
};

vi.mock('@/api/client', () => ({ apiRequest: vi.fn((url) => api(url)) }));
vi.mock('@/api/blacklist', () => ({ checkVehicleBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

const FIELD_CFG = {
  number: { visible: true, required: false },
  mark: { visible: true, required: false },
  unloading_places: { visible: true, required: false },
  passage_tables: { visible: true, required: false },
};

describe('VehicleForm - предупреждения мест (свободный текст + окна) в noticeGroups (#1183)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('нет выбранных мест -> noticeGroups пуст', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });

  it('группа несёт свободный warning и активное окно, скрывает отключённое', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    const groups = w.vm.noticeGroups;
    expect(groups).toHaveLength(1);
    expect(groups[0].name).toBe('Пост №72');
    expect(groups[0].free).toBe('Только малогабарит');
    expect(groups[0].windows).toContain('Активное окно');
    expect(groups[0].windows).not.toContain('Отключённое окно');
  });

  it('место без предупреждений - не попадает в noticeGroups', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [2];
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });

  it('окно фильтруется по warningNow (тикающий момент)', async () => {
    const timed = [{
      id: 1, name: 'Пост №72', status: 'active', warning: null,
      warning_windows: [{ id: 1, day_of_week: 0, time_from: '09:00', time_to: '10:00', is_next_day: false, message: 'Окно 9-10', is_active: true }],
    }];
    apiRequest.mockImplementation((url) =>
      url === '/unload-places'
        ? Promise.resolve({ ok: true, json: async () => timed })
        : Promise.resolve({ ok: true, json: async () => [] }));

    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];

    w.vm.warningNow = new Date(2026, 6, 13, 9, 30); // Пн в окне
    await flushPromises();
    expect(w.vm.noticeGroups[0].windows).toContain('Окно 9-10');

    w.vm.warningNow = new Date(2026, 6, 13, 11, 0); // Пн вне окна
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });

  it('warning таблицы проезда берётся из table.warning (обёрнутый DTO)', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedPassageTables = [10];
    await flushPromises();

    const groups = w.vm.noticeGroups;
    expect(groups).toHaveLength(1);
    expect(groups[0].name).toBe('Проезд Ворота-1');
    expect(groups[0].free).toBe('Высота до 3 м');
  });
});
