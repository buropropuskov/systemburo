import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { usePermissionsStore } from '@/stores/permissions';
import VehicleDetailsModal from '../VehicleDetailsModal.vue';
import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import CarHistoryModal from '@/components/CarHistoryModal.vue';
import EmployeeHistoryModal from '../EmployeeHistoryModal.vue';
import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

describe('detail history permissions', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks(); });

  it.each([VehicleDetailsModal, EmployeeDetailsModal])('requires the history key in blacklist context', (component) => {
    const store = usePermissionsStore();
    store.effective = { 'page.admin.blacklist': { value: 'allow' } };
    expect(component.computed.canSeeFullHistory.call({ source: 'blacklist' })).toBe(false);
    store.effective['detail.full_history'] = { value: 'allow' };
    expect(component.computed.canSeeFullHistory.call({ source: 'blacklist' })).toBe(true);
    expect(component.computed.canSeeEntryExit.call({ source: 'trash' })).toBe(false);
  });

  it.each([[VehicleDetailsModal, 'loadCarHistory'], [EmployeeDetailsModal, 'loadHistory']])('does not request hidden passage history', async (component, method) => {
    const context = { canSeeEntryExit: false, history: [{ action_type: 'entry' }] };
    await component.methods[method].call(context);
    expect(context.history).toEqual([]);
    expect(apiRequest).not.toHaveBeenCalled();
  });

  it.each([CarHistoryModal, EmployeeHistoryModal])('does not request full history without its capability', async (component) => {
    const context = { history: [{ action_type: 'update' }] };
    await component.methods.loadHistory.call(context);
    expect(context.history).toEqual([]);
    expect(apiRequest).not.toHaveBeenCalled();
  });
});
