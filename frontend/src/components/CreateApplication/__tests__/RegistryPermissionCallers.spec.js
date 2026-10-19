import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { usePermissionsStore } from '@/stores/permissions';
import { apiRequest } from '@/api/client';
import CreateApplication from '../CreateApplication.vue';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(), createExtendedTimeoutSignal: vi.fn() }));

beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks(); });
function grant(kind, read, write) {
  usePermissionsStore().effective = {
    [`entity.${kind}.read`]: { value: read ? 'allow' : 'deny' },
    [`entity.${kind}.write`]: { value: write ? 'allow' : 'deny' },
  };
}
describe.each([
  ['cars', ExistingCarsModal, 'Cars', 'Vehicles'],
  ['employees', ExistingEmployeesModal, 'Employees', 'Employees'],
])('optional %s registry requests', (kind, Modal, word, bindingWord) => {
  it('does not load a denied catalog and clears stale rows', async () => {
    grant(kind, false, true);
    const vm = { [`filtered${word}`]: [{ id: 1 }], [`displayed${word}`]: [{ id: 1 }] };
    await Modal.methods[`load${word}ByFilter`].call(vm, 'all');
    expect(apiRequest).not.toHaveBeenCalled();
    expect(vm[`filtered${word}`]).toEqual([]); expect(vm[`displayed${word}`]).toEqual([]);
    expect(vm[`loading${word}`]).toBe(false);
  });
  it.each([[false, true], [true, false], [false, false]])('skips optional enrichment without read/write: %s/%s', async (read, write) => {
    grant(kind, read, write);
    const vm = { canCreateRegistry: CreateApplication.methods.canCreateRegistry };
    expect(vm.canCreateRegistry(kind)).toBe(false);
    expect(await CreateApplication.methods[`loadExisting${bindingWord}`].call(vm)).toEqual([]);
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('loads available catalog silently if permission changes during the request', async () => {
    grant(kind, true, true);
    apiRequest.mockResolvedValue({ ok: false, status: 403 });
    const vm = { canCreateRegistry: CreateApplication.methods.canCreateRegistry };
    expect(await CreateApplication.methods[`loadExisting${bindingWord}`].call(vm)).toEqual([]);
    expect(apiRequest).toHaveBeenCalledWith(`/unique-${kind}?filter_type=all`, expect.objectContaining({ silent403: true }));
  });
});
it('submits without optional registry writes after permissions are revoked', async () => {
  const vm = {
    canCreateRegistry: CreateApplication.methods.canCreateRegistry,
    newVehiclesToBind: [{ plateNumber: 'SYN' }], newEmployeesToBind: [{ lastName: 'SYN' }],
    closeBindingModal: vi.fn(), sendCompleteApplication: vi.fn(),
  };
  await CreateApplication.methods.confirmBinding.call(vm, { vehicles: { hasVehiclesForBinding: true }, employees: { hasEmployeesForBinding: true } });
  expect(apiRequest).not.toHaveBeenCalled(); expect(vm.sendCompleteApplication).toHaveBeenCalledOnce();
  expect(vm.isBindingActionInProgress).toBe(false);
});
