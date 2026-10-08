import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import EntityPeriodEditor, { periodEditorEntityID } from '../EntityPeriodEditor.vue';
import EntityPeriodModal from '../EntityPeriodModal.vue';
import EmployeeDetailsModal from '../CreateApplication/EmployeeDetailsModal.vue';
import VehicleDetailsModal from '../CreateApplication/VehicleDetailsModal.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const wrappers = [];
function editor(props = {}, realModal = false) {
  const wrapper = mount(EntityPeriodEditor, {
    props: { show: true, kind: 'employee', source: 'application', entity: { id: 17 }, ...props },
    global: { stubs: { Teleport: true, ...(realModal ? {} : { EntityPeriodModal: true }) } },
  });
  wrappers.push(wrapper);
  return wrapper;
}

describe('EntityPeriodEditor #2665', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().$patch({ mode: 'normal', effective: { 'detail.period.change': { value: 'allow' } } });
    apiRequest.mockReset();
  });
  afterEach(() => wrappers.splice(0).forEach(wrapper => wrapper.unmount()));

  it.each([
    ['employee', 'employeesview', { id: 999, activeEmployeeId: 17 }, 17],
    ['car', 'carsview', { id: 999, activeCarId: 18 }, 18],
    ['employee', 'peopletable', { id: 17 }, 17],
    ['car', 'carstable', { id: 18 }, 18],
    ['employee', 'application', { id: 17 }, 17],
    ['car', 'application', { id: 18 }, 18],
    ['employee', 'employeesview', { id: 999 }, null],
    ['car', 'carsview', { id: 999, plateNumber: 'TEST' }, null],
    ['employee', 'blacklist', { id: 999, activeEmployeeId: 17 }, null],
    ['car', 'blacklist', { id: 999, activeCarId: 18 }, null],
    ['employee', 'employeeslist', { id: 123456 }, null],
    ['car', 'vehicleslist', { id: 123456 }, null],
    ['car', 'general', { id: 123456, isExisting: true }, null],
    ['employee', 'application', { id: 17, isDraft: true }, null],
    ['car', 'application', { id: 18, isPending: true }, null],
    ['employee', 'application', { id: '17' }, null],
    ['car', 'application', { id: -1 }, null],
  ])('resolves only a persisted basis ID: %s/%s', (kind, source, entity, expected) => {
    expect(periodEditorEntityID(kind, source, entity)).toBe(expected);
  });

  it('does not mount the modal or request an API for registry Unique IDs without a basis', () => {
    const wrapper = editor({ source: 'employeesview', entity: { id: 999, passport_series_number: 'synthetic' } });
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(false);
    expect(wrapper.findComponent(EntityPeriodModal).exists()).toBe(false);
    expect(apiRequest).not.toHaveBeenCalled();
  });

  it('uses the active row ID, optional table context and unchanged result event', async () => {
    const wrapper = editor({ kind: 'car', source: 'carsview', entity: { id: 999, activeCarId: 18 }, tableId: 5 });
    await wrapper.find('[data-testid="entity-period-open"]').trigger('click');
    const modal = wrapper.findComponent(EntityPeriodModal);
    expect(modal.props()).toMatchObject({ kind: 'car', entityID: 18, tableID: 5, show: true });
    const result = { entity_id: 18, attachment_id: 31, period_mode: 'individual', approvals_reset: false };
    modal.vm.$emit('changed', result);
    await flushPromises();
    expect(wrapper.emitted('changed')).toEqual([[result]]);
    expect(wrapper.findComponent(EntityPeriodModal).exists()).toBe(false);
  });

  it('requires the managed permission, honors deny and closes after revocation', async () => {
    const wrapper = editor();
    await wrapper.find('[data-testid="entity-period-open"]').trigger('click');
    expect(wrapper.findComponent(EntityPeriodModal).exists()).toBe(true);
    usePermissionsStore().$patch({ mode: 'admin', denied: new Set(['detail.period.change']) });
    await flushPromises();
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(false);
    expect(wrapper.findComponent(EntityPeriodModal).exists()).toBe(false);
    // Object $patch deep-merges maps: {} would retain the previous allow.
    // Replace state exactly as fetchPermissions does after server revocation.
    usePermissionsStore().mode = 'normal';
    usePermissionsStore().effective = {};
    await flushPromises();
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(false);
  });

  it.each([{ readonly: true }, { show: false }, { source: 'employeeslist' }, { tableId: 0 }])('hides editing for readonly/closed/draft/invalid table: %j', props => {
    expect(editor(props).find('[data-testid="entity-period-open"]').exists()).toBe(false);
  });

  it('resets the nested editor when a different persisted basis is selected', async () => {
    const wrapper = editor();
    await wrapper.find('[data-testid="entity-period-open"]').trigger('click');
    await wrapper.setProps({ entity: { id: 19 } });
    expect(wrapper.findComponent(EntityPeriodModal).exists()).toBe(false);
  });

  it('defers lifecycle authorization to real Inspect instead of assuming application means Work', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 400, json: async () => ({ message: 'В этом состоянии срок изменять нельзя' }) });
    const wrapper = editor({}, true);
    expect(apiRequest).not.toHaveBeenCalled();
    await wrapper.find('[data-testid="entity-period-open"]').trigger('click');
    await flushPromises();
    expect(apiRequest).toHaveBeenCalledExactlyOnceWith('/employees/17/period');
    expect(wrapper.findComponent(EntityPeriodModal).find('[role="alert"]').text()).toContain('В этом состоянии срок изменять нельзя');
    expect(wrapper.findComponent(EntityPeriodModal).find('[data-testid="entity-period-save"]').attributes('disabled')).toBeDefined();
  });

  it('both card contracts expose one period-changed event and optional table context', () => {
    for (const card of [EmployeeDetailsModal, VehicleDetailsModal]) {
      const header = card.components.DetailHeaderActions;
      expect(header.components.EntityPeriodEditor).toBe(EntityPeriodEditor);
      expect(header.emits).toContain('period-changed');
      expect(header.props.periodTableId.default).toBeNull();
      expect(card.emits).toContain('period-changed');
      expect(card.props.periodTableId.default).toBeNull();
    }
  });
});
