import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import VehicleDetailsModal from '../VehicleDetailsModal.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Карточки просмотра добавляемого человека/машины в списках заявки (EmployeesList/
// VehiclesList) открывают эти модалки с readonly=true: кнопки действий (ЧС, открыть
// заявку) скрыты, а раздел «Документы» сотрудника показывается всегда (свои данные
// формы). Вспомогательные API на show=true глушим.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  createPersonBlacklist: vi.fn().mockResolvedValue({}),
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
  createVehicleBlacklist: vi.fn().mockResolvedValue({}),
}));
vi.mock('exceljs', () => ({ default: {} }));

const stubs = {
  teleport: true,
  EmployeeHistoryModal: true,
  CarHistoryModal: true,
  TableInfoModal: true,
  UnloadingPlaceModal: true,
  AddToBlacklistModal: true,
};

const PASSPORT_LABEL = 'Серия и номер паспорта';

function mountEmployee(props, setupStore) {
  setActivePinia(createPinia());
  setupStore(usePermissionsStore());
  return mount(EmployeeDetailsModal, {
    props: {
      show: true,
      employee: { id: 1, last_name: 'Иваноф', first_name: 'Иван', passport_series_number: '1234 567890', target_tables: [] },
      source: 'employeeslist',
      ...props,
    },
    global: { stubs },
  });
}

function mountVehicle(props, setupStore) {
  setActivePinia(createPinia());
  setupStore(usePermissionsStore());
  return mount(VehicleDetailsModal, {
    props: {
      show: true,
      vehicle: { id: 1, plateNumber: 'А123ВС777', mark: 'Toyota', unloadPlaces: [] },
      ...props,
    },
    global: { stubs },
  });
}

describe('Карточки просмотра в списках заявки (readonly)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('employeeslist + readonly: кнопка «В ЧС» скрыта даже у super', () => {
    const w = mountEmployee({ readonly: true }, (s) => { s.mode = 'super'; });
    expect(w.find('.blacklist-add-btn').exists()).toBe(false);
  });

  it('employeeslist без readonly: кнопка «В ЧС» у super видна (readonly - то, что её гасит)', () => {
    const w = mountEmployee({ readonly: false }, (s) => { s.mode = 'super'; });
    expect(w.find('.blacklist-add-btn').exists()).toBe(true);
  });

  it('employeeslist: документы сотрудника видны без явного права (свои данные формы)', () => {
    const w = mountEmployee({ readonly: true }, (s) => {
      s.mode = 'normal';
      s.effective = {};
    });
    expect(w.text()).toContain(PASSPORT_LABEL);
  });

  it('vehicle readonly: кнопка «В ЧС» скрыта даже у super', () => {
    const w = mountVehicle({ readonly: true }, (s) => { s.mode = 'super'; });
    expect(w.find('.blacklist-add-btn').exists()).toBe(false);
  });

  it('vehicle без readonly: кнопка «В ЧС» у super видна', () => {
    const w = mountVehicle({ readonly: false }, (s) => { s.mode = 'super'; });
    expect(w.find('.blacklist-add-btn').exists()).toBe(true);
  });
});
