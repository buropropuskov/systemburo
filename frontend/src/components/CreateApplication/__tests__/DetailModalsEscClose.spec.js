import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import VehicleDetailsModal from '../VehicleDetailsModal.vue';

// Закрытие карточек Т/С и сотрудника по Escape (фон закрывается overlay-обработчиком).
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

const EMP = { id: 1, last_name: 'Иванов', first_name: 'Иван', target_tables: [] };
const CAR = { id: 1, plateNumber: 'А123ВС777', mark: 'Toyota', unloadPlaces: [] };

const pressEsc = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

let w;
beforeEach(() => setActivePinia(createPinia()));
// Размонтируем - иначе document-listener модалки утекает в следующий тест.
afterEach(() => { if (w) { w.unmount(); w = null; } });

describe('DetailModals - закрытие по Escape', () => {
  it('EmployeeDetailsModal: Escape при show=true -> close', async () => {
    w = mount(EmployeeDetailsModal, { props: { show: true, employee: EMP, source: 'employeesview' }, global: { stubs } });
    pressEsc();
    await w.vm.$nextTick();
    expect(w.emitted('close')).toBeTruthy();
  });

  it('VehicleDetailsModal: Escape при show=true -> close', async () => {
    w = mount(VehicleDetailsModal, { props: { show: true, vehicle: CAR }, global: { stubs } });
    pressEsc();
    await w.vm.$nextTick();
    expect(w.emitted('close')).toBeTruthy();
  });

  it('VehicleDetailsModal: Escape при show=false -> не закрывает', async () => {
    w = mount(VehicleDetailsModal, { props: { show: false, vehicle: CAR }, global: { stubs } });
    pressEsc();
    await w.vm.$nextTick();
    expect(w.emitted('close')).toBeFalsy();
  });
});
