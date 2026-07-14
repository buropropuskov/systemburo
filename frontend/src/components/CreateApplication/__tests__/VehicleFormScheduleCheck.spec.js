import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { apiRequest } from '@/api/client';
import VehicleForm from '../VehicleForm.vue';

// #1183 S5: при добавлении машины сверяем расписание (time_slots) выбранных мест с
// сроком заявки (prop entryPeriod) и предупреждаем неблокирующе, если место закрыто
// на границе срока. 2026-07-13 = понедельник = day_of_week 0.

// Место с расписанием Пн 09:00-18:00 (плоский DTO /unload-places -> time_slots в корне).
const PLACES = [
  {
    id: 1, name: 'Пост №72', status: 'active', warning: null, warning_windows: [],
    time_slots: [
      { id: 1, day_of_week: 0, open_time: '09:00', close_time: '18:00', is_next_day: false, is_active: true },
    ],
  },
];

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

const api = (url) => {
  if (url === '/unload-places') return Promise.resolve({ ok: true, json: async () => PLACES });
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

// Пн 08:00-20:00: въезд ДО открытия, выезд ПОСЛЕ закрытия -> обе границы вне графика.
const OUTSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '08:00', time_to: '20:00' };
// Пн 10:00-17:00: полностью в графике.
const INSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '10:00', time_to: '17:00' };

const mountForm = (entryPeriod) =>
  mount(VehicleForm, { props: { fieldConfig: FIELD_CFG, entryPeriod }, attachTo: document.body });

describe('VehicleForm - авто-проверка расписания против срока (#1183 S5)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('баннер предупреждает, если срок выходит за график работы места', async () => {
    const w = mountForm(OUTSIDE);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    const banner = w.find('[data-testid="vehicle-place-warnings"]');
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain('Пост №72');
    expect(banner.text()).toContain('въезда');
    expect(banner.text()).toContain('выезда');
  });

  it('срок внутри графика -> баннера нет', async () => {
    const w = mountForm(INSIDE);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    expect(w.find('[data-testid="vehicle-place-warnings"]').exists()).toBe(false);
  });

  it('без срока (entryPeriod=null) расписание не проверяется', async () => {
    const w = mountForm(null);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    expect(w.find('[data-testid="vehicle-place-warnings"]').exists()).toBe(false);
  });

  it('addVehicle шлёт notify type=warning с предупреждением расписания', async () => {
    const w = mountForm(OUTSIDE);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    w.vm.addVehicle();
    await flushPromises();

    expect(w.emitted('vehicle-added')).toBeTruthy();
    const warns = warnCalls();
    expect(warns).toHaveLength(1);
    expect(warns[0].prefix).toContain('Пост №72');
    expect(warns[0].bold).toContain('графику работы');
  });
});
