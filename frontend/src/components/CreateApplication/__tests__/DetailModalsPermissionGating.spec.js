import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import VehicleDetailsModal from '../VehicleDetailsModal.vue';
import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Карточки на show=true дёргают вспомогательные API (статус/история) - глушим, нас
// интересует только гейтинг кнопки «Полная история» и секции истории проходов по праву.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/blacklist', () => ({
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  listPersonBlacklist: vi.fn().mockResolvedValue([]),
}));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));
vi.mock('exceljs', () => ({ default: {} }));

const stubs = {
  teleport: true,
  UnloadPlaceModal: true,
  CarHistoryModal: true,
  EmployeeHistoryModal: true,
  TableInfoModal: true,
  AddToBlacklistModal: true,
  LoaderSpinner: true,
};

/** Задаёт режим/права в свежем сторе до монтирования модалки. */
function seedPerms({ mode = 'normal', allow = [] } = {}) {
  const perms = usePermissionsStore();
  perms.mode = mode;
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
}

function mountVehicle(props = {}) {
  return mount(VehicleDetailsModal, {
    props: { show: true, vehicle: { id: 1, plateNumber: 'А123ВС', mark: 'Toyota', unloadPlaces: [] }, source: 'application', showCarFeatures: true, ...props },
    global: { stubs },
  });
}

function mountEmployee(props = {}) {
  return mount(EmployeeDetailsModal, {
    props: { show: true, employee: { id: 1, last_name: 'Иваноф', first_name: 'Иван', middle_name: 'Иванович', target_tables: [] }, source: 'application', ...props },
    global: { stubs },
  });
}

const hasSection = (w, title) => w.findAll('.section-title').some(h => h.text().includes(title));

describe('VehicleDetailsModal — гейтинг истории по detail.* (срез 4)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('normal с detail.full_history/entry_exit_history: кнопка истории и секция проходов видны', () => {
    seedPerms({ allow: ['detail.full_history', 'detail.entry_exit_history'] });
    const w = mountVehicle();
    expect(w.find('.history-btn').exists()).toBe(true);
    expect(hasSection(w, 'История въездов и выездов')).toBe(true);
  });

  it('normal без прав: кнопка истории и секция проходов скрыты (тумблер реально гейтит)', () => {
    seedPerms({ allow: [] });
    const w = mountVehicle();
    expect(w.find('.history-btn').exists()).toBe(false);
    expect(hasSection(w, 'История въездов и выездов')).toBe(false);
  });

  it('super видит историю без явных грантов', () => {
    seedPerms({ mode: 'super' });
    const w = mountVehicle();
    expect(w.find('.history-btn').exists()).toBe(true);
    expect(hasSection(w, 'История въездов и выездов')).toBe(true);
  });

  it('контекст «Корзина»: normal без detail.full_history — кнопка истории скрыта', () => {
    seedPerms({ allow: [] });
    const w = mountVehicle({ source: 'trash' });
    expect(w.find('.history-btn').exists()).toBe(false);
  });

  it('контекст «Корзина»: с detail.full_history кнопка истории видна (контекст не ломает гейт)', () => {
    seedPerms({ allow: ['detail.full_history'] });
    const w = mountVehicle({ source: 'trash' });
    expect(w.find('.history-btn').exists()).toBe(true);
  });
});

describe('EmployeeDetailsModal — гейтинг истории по detail.* (срез 4)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('normal с detail.full_history/entry_exit_history: кнопка истории и секция проходов видны', () => {
    seedPerms({ allow: ['detail.full_history', 'detail.entry_exit_history'] });
    const w = mountEmployee();
    expect(w.find('.history-btn').exists()).toBe(true);
    expect(hasSection(w, 'История проходов')).toBe(true);
  });

  it('normal без прав: кнопка истории и секция проходов скрыты', () => {
    seedPerms({ allow: [] });
    const w = mountEmployee();
    expect(w.find('.history-btn').exists()).toBe(false);
    expect(hasSection(w, 'История проходов')).toBe(false);
  });

  it('super видит историю без явных грантов', () => {
    seedPerms({ mode: 'super' });
    const w = mountEmployee();
    expect(w.find('.history-btn').exists()).toBe(true);
    expect(hasSection(w, 'История проходов')).toBe(true);
  });

  it('контекст «Список в заявке» (employeeslist): история скрыта даже у super — контекст-гард жив', () => {
    seedPerms({ mode: 'super' });
    const w = mountEmployee({ source: 'employeeslist' });
    expect(w.find('.history-btn').exists()).toBe(false);
    expect(hasSection(w, 'История проходов')).toBe(false);
  });
});
