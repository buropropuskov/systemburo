import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { apiRequest } from '@/api/client';
import VehicleForm from '../VehicleForm.vue';

// #1183 polish: форма собирает noticeGroups (место -> {free, windows, schedule}) и
// эмитит notices-change наверх в единую панель. Расписание сверяется по ПЕРЕСЕЧЕНИЮ
// окна пребывания срока с графиком. 2026-07-13 = понедельник (day_of_week 0).

const PLACES = [
  {
    id: 1, name: 'Ворота Маугли', status: 'active', warning: null, warning_windows: [],
    time_slots: [
      { id: 1, day_of_week: 0, open_time: '10:00', close_time: '12:00', is_next_day: false, is_active: true },
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

// Пн 13:00-16:00: пребывание ПОСЛЕ закрытия (12:00) -> вне графика.
const OUTSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '13:00', time_to: '16:00' };
// Пн 10:30-11:30: внутри графика.
const INSIDE = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '10:30', time_to: '11:30' };

const wait = (ms) => new Promise((r) => setTimeout(r, ms));

const mountForm = (entryPeriod) =>
  mount(VehicleForm, { props: { fieldConfig: FIELD_CFG, entryPeriod }, attachTo: document.body });

describe('VehicleForm - авто-проверка расписания против окна пребывания (#1183 polish)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiRequest.mockImplementation(api);
  });

  it('окно пребывания вне графика -> группа с schedule.anyClosed и режимом дня', async () => {
    const w = mountForm(OUTSIDE);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();

    const groups = w.vm.noticeGroups;
    expect(groups).toHaveLength(1);
    expect(groups[0].name).toBe('Ворота Маугли');
    expect(groups[0].schedule.anyClosed).toBe(true);
    expect(groups[0].schedule.presence).toBe('13:00—16:00');
    expect(groups[0].schedule.days[0].hours).toEqual(['10:00—12:00']);
    expect(groups[0].schedule.days[0].open).toBe(false);
  });

  it('окно пребывания внутри графика -> предупреждения нет', async () => {
    const w = mountForm(INSIDE);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });

  it('без срока (entryPeriod=null) расписание не проверяется', async () => {
    const w = mountForm(null);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();
    expect(w.vm.noticeGroups).toHaveLength(0);
  });

  it('эмитит notices-change наверх (после дебаунса)', async () => {
    const w = mountForm(OUTSIDE);
    await flushPromises();
    w.vm.selectedUnloadingPlaces = [1];
    await flushPromises();
    await wait(200);
    const emitted = w.emitted('notices-change');
    expect(emitted).toBeTruthy();
    const last = emitted[emitted.length - 1][0];
    expect(last.some((g) => g.schedule && g.schedule.anyClosed)).toBe(true);
  });
});
