import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ManualAttachCardAction from '../ManualAttachCardAction.vue';
import ManualAttachExistingModal from '../ManualAttachExistingModal.vue';
import { getManualAttachContext } from '@/api/manualAttachSingle';
import { getAttachableApplications } from '@/api/applications';
import { usePermissionsStore } from '@/stores/permissions';

vi.mock('@/api/manualAttachSingle', () => ({ getManualAttachContext: vi.fn(), previewManualAttachSingle: vi.fn(), executeManualAttachSingle: vi.fn() }));
vi.mock('@/api/applications', () => ({ getAttachableApplications: vi.fn(), getApplicationAttachments: vi.fn() }));
const wrappers = [];
const context = id => ({ entity_id: id, entity_kind: 'car', attachment_id: 8, is_manual: true, application_id: null,
  period_mode: 'inherit', effective_period: { bounded: false }, requires_period_choice: true, can_assign_period: true });
function create(props = {}) {
  const wrapper = mount(ManualAttachCardAction, { props: { show: true, kind: 'car', entityID: 17, tableID: 4, ...props }, global: { stubs: { Teleport: true } } });
  wrappers.push(wrapper);
  return wrapper;
}

describe('manual attach card action #2665', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().$patch({ mode: 'normal', effective: { 'page.admin': { value: 'allow' } } });
    getManualAttachContext.mockReset().mockImplementation(async (_, id) => context(id));
    getAttachableApplications.mockReset().mockResolvedValue([]);
  });
  afterEach(() => wrappers.splice(0).forEach(w => w.unmount()));
  it('shows only after fresh server context and opens the real modal with the actual ID', async () => {
    const wrapper = create();
    expect(wrapper.find('[data-testid="manual-attach-card-action"]').exists()).toBe(false);
    await flushPromises();
    expect(getManualAttachContext).toHaveBeenCalledWith('car', 17, 4);
    await wrapper.find('[data-testid="manual-attach-card-action"]').trigger('click');
    await flushPromises();
    expect(wrapper.findComponent(ManualAttachExistingModal).props()).toMatchObject({ show: true, entityID: 17, tableID: 4 });
    expect(getManualAttachContext).toHaveBeenCalledTimes(2);
  });
  it.each([{ show: false }, { readonly: true }, { entityID: 0 }, { tableID: null }])('does not fetch ineligible card %j', async props => {
    const wrapper = create(props);
    await flushPromises();
    expect(getManualAttachContext).not.toHaveBeenCalled();
    expect(wrapper.find('button').exists()).toBe(false);
  });
  it('does not bypass page.admin and closes when it is revoked', async () => {
    const wrapper = create();
    await flushPromises();
    await wrapper.find('[data-testid="manual-attach-card-action"]').trigger('click');
    usePermissionsStore().effective = {};
    await flushPromises();
    expect(wrapper.find('button').exists()).toBe(false);
    expect(wrapper.vm.open).toBe(false);
  });
  it('ignores late context after switching the open entity', async () => {
    let resolve;
    getManualAttachContext.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    const wrapper = create();
    await wrapper.setProps({ entityID: 18 });
    await flushPromises();
    resolve(context(17));
    await flushPromises();
    expect(wrapper.vm.context.entity_id).toBe(18);
  });
  it('hides a denied/nonmanual context, but exposes a retry for connection failures', async () => {
    getManualAttachContext.mockRejectedValueOnce(Object.assign(new Error('Denied'), { status: 403 }));
    const denied = create();
    await flushPromises();
    expect(denied.find('button').exists()).toBe(false);
    getManualAttachContext.mockRejectedValueOnce(new Error('Connection failed'));
    const failed = create();
    await flushPromises();
    await failed.find('[data-testid="manual-attach-card-retry"]').trigger('click');
    await flushPromises();
    expect(failed.find('[data-testid="manual-attach-card-action"]').exists()).toBe(true);
  });
});
