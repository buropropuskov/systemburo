import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import VehicleDetailsModal from '../VehicleDetailsModal.vue';
import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import { usePermissionsStore } from '@/stores/permissions';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false, json: async () => [] }) }));
vi.mock('@/api/blacklist', () => ({ listVehicleBlacklist: vi.fn().mockResolvedValue([]), checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }), listPersonBlacklist: vi.fn().mockResolvedValue([]) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));
vi.mock('exceljs', () => ({ default: {} }));
const wrappers = [];
const status = { can_revert: true, last_mark_table_id: 7, territory_status: 1, server_now: '2030-01-01T12:20:00Z', passage_state: { last_event_id: 9, last_event_kind: 'entry', entry_at: '2030-01-01T12:00:00Z' } };
const stubs = { teleport: true, CarHistoryModal: true, EmployeeHistoryModal: true, UnloadPlaceModal: true, TableInfoModal: true, AddToBlacklistModal: true };
async function open(source, readonly = false) {
  const person = source === 'peopletable';
  const props = { show: true, source, readonly, periodTableId: 7, showCarFeatures: true, ...(person ? { employee: { id: 17, first_name: 'Тест', last_name: 'Запись' } } : { vehicle: { id: 17, plateNumber: 'TEST-17', mark: 'Тест', unloadPlaces: [] } }) };
  const w = mount(person ? EmployeeDetailsModal : VehicleDetailsModal, { props, global: { stubs } }); wrappers.push(w);
  await flushPromises(); await w.setData({ currentRevertStatus: status }); await flushPromises(); return w;
}
beforeEach(() => { setActivePinia(createPinia()); usePermissionsStore().mode = 'admin'; });
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); });
describe('cancellation belongs to the status section', () => {
  it.each(['carstable', 'peopletable', 'facttable'])('keeps the action beside the status in %s', async source => {
    const w = await open(source);
    const action = w.get('[data-testid="passage-card-revert"]').element;
    const section = action.closest('.details-section');
    expect(section.querySelector('h4').textContent.trim()).toBe('Статус');
    expect(section.querySelector('.status-badge')).not.toBeNull();
    expect(w.findAll('[data-testid="passage-card-revert"]')).toHaveLength(1);
  });
  it('does not make a read-only card actionable after the relocation', async () => {
    const w = await open('carstable', true);
    expect(w.find('[data-testid="passage-card-revert"]').exists()).toBe(false);
  });
});
