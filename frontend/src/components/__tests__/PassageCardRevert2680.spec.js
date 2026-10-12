import { beforeEach, describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick as tick } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import PassageCardRevertAction from '../PassageCardRevertAction.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { usePassageRevertStore } from '@/stores/passageRevert';
const stamp = '2030-01-01T12:00:00Z';
function props(extra = {}) {
  return { active: true, kind: 'car', entityID: 17, tableID: 7, subject: 'Тестовая машина',
    status: { can_revert: true, last_mark_table_id: 7, territory_status: 1, server_now: '2030-01-01T12:20:00Z',
      passage_state: { last_event_kind: 'entry', last_event_id: 9, entry_at: stamp } }, ...extra };
}
describe('explicit administrative cancellation', () => {
  beforeEach(() => { setActivePinia(createPinia()); usePermissionsStore().mode = 'admin'; });
  it('opens the existing reason form with the exact last event and refresh callbacks', async () => {
    const wrapper = mount(PassageCardRevertAction, { props: props() });
    await wrapper.get('button').trigger('click');
    const request = usePassageRevertStore().request;
    expect(request).toMatchObject({ kind: 'cars', id: 17, tableId: 7, direction: 'entry', expectedLastEventID: 9 });
    request.onRefresh(); expect(wrapper.emitted('changed')).toHaveLength(1);
    wrapper.unmount();
  });
  it('does not expose cancellation through a historical/read-only card', () => {
    const wrapper = mount(PassageCardRevertAction, { props: props({ active: false }) });
    expect(wrapper.find('button').exists()).toBe(false); wrapper.unmount();
  });
  it('requires administrator access, a server grant and the same post', async () => {
    const wrapper = mount(PassageCardRevertAction, { props: props({ tableID: 8 }) });
    expect(wrapper.find('button').exists()).toBe(false);
    await wrapper.setProps(props());
    usePermissionsStore().mode = 'normal'; await tick();
    expect(wrapper.find('button').exists()).toBe(false);
    usePermissionsStore().mode = 'admin'; await wrapper.setProps({ status: { ...props().status, can_revert: false } });
    expect(wrapper.find('button').exists()).toBe(false); wrapper.unmount();
  });
  it('does not duplicate the fresh quick action', () => {
    const wrapper = mount(PassageCardRevertAction, { props: props({ status: { ...props().status, server_now: stamp } }) });
    expect(wrapper.find('button').exists()).toBe(false); wrapper.unmount();
  });
});
