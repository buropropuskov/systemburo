import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { apiRequest } from '@/api/client';
import VehicleForm from '../VehicleForm.vue';

// #1183 S4: при добавлении машины показываем предупреждения выбранных мест -
// свободный текст (warning) + окна, активные сейчас (warning_windows, is_active).
// Окна в фикстурах "каждый день / весь день" -> активны в любой момент (детерминизм).

// /unload-places отдаёт плоский объект, форма присваивает as-is -> warning-поля на верхнем уровне.
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

const warnCalls = () => notifyMock.mock.calls.map((c) => c[0]).filter((a) => a && a.type === 'warning');

describe('VehicleForm - предупреждения мест при добавлении (#1183 S4)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('баннера нет, пока не выбрано место с предупреждением', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    expect(w.find('[data-testid="vehicle-place-warnings"]').exists()).toBe(false);
  });

  it('баннер показывает свободный warning и активное окно, скрывает отключённое', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();

    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    const banner = w.find('[data-testid="vehicle-place-warnings"]');
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain('Пост №72');
    expect(banner.text()).toContain('Только малогабарит');
    expect(banner.text()).toContain('Активное окно');
    expect(banner.text()).not.toContain('Отключённое окно');
  });

  it('addVehicle шлёт notify type=warning по местам с предупреждением', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    w.vm.addVehicle();
    await flushPromises();

    expect(w.emitted('vehicle-added')).toBeTruthy();
    const warns = warnCalls();
    expect(warns).toHaveLength(1);
    expect(warns[0].prefix).toContain('Пост №72');
    expect(warns[0].bold).toContain('Только малогабарит');
    expect(warns[0].bold).toContain('Активное окно');
  });

  it('место без предупреждений - баннера нет и notify type=warning не шлётся', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [2];
    await flushPromises();

    expect(w.find('[data-testid="vehicle-place-warnings"]').exists()).toBe(false);
    w.vm.addVehicle();
    await flushPromises();
    expect(warnCalls()).toHaveLength(0);
  });

  it('warning таблицы проезда берётся из table.warning (обёрнутый DTO)', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();
    w.vm.selectedPassageTables = [10];
    await flushPromises();

    const banner = w.find('[data-testid="vehicle-place-warnings"]');
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain('Проезд Ворота-1');
    expect(banner.text()).toContain('Высота до 3 м');
  });
});
