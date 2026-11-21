import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import Component from '../ApplicationAttachmentWithChanges.vue';
const mocks = vi.hoisted(() => ({ load: vi.fn(), acknowledge: vi.fn(), notify: vi.fn() }));
vi.mock('@/utils/applicationChanges', () => ({ loadApplicationChanges: mocks.load, acknowledgeApplicationChanges: mocks.acknowledge }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: mocks.notify }) }));
const child = { name: 'ApplicationAttachmentDetail', props: ['changes'], template: '<div />' };
function render() { return mount(Component, { props: { applicationId: 7, attachment: { id: 1 } }, global: { stubs: { ApplicationAttachmentDetail: child } } }); }

describe('application change attention lifecycle', () => {
  beforeEach(() => vi.clearAllMocks());
  it('switches highlighted changes with the selected attachment', async () => {
    mocks.load.mockResolvedValue([{ key: '1:0', attachment_id: 1 }, { key: '1:1', attachment_id: 2 }]);
    const wrapper = render(); await flushPromises();
    expect(wrapper.findComponent(child).props('changes').map(change => change.key)).toEqual(['1:0']);
    await wrapper.setProps({ attachment: { id: 2 } }); await flushPromises();
    expect(wrapper.findComponent(child).props('changes').map(change => change.key)).toEqual(['1:1']);
    wrapper.unmount();
  });
  it('ignores a late response from the previous application', async () => {
    let completeOld;
    mocks.load.mockImplementationOnce(() => new Promise(resolve => { completeOld = resolve; }))
      .mockResolvedValueOnce([{ key: '8:0', attachment_id: 2 }]);
    const wrapper = render();
    await wrapper.setProps({ applicationId: 8, attachment: { id: 2 } }); await flushPromises();
    completeOld([{ key: '7:0', attachment_id: 1 }]); await flushPromises();
    expect(wrapper.findComponent(child).props('changes').map(change => change.key)).toEqual(['8:0']);
    wrapper.unmount();
  });
  it('opens the affected attachment once without fighting subsequent manual selection', async () => {
    mocks.load.mockResolvedValue([{ key: '1:0', attachment_id: 2 }]);
    const focusAttachment = vi.fn();
    const wrapper = mount(Component, { props: { applicationId: 7, attachment: { id: 1 }, attachments: [{ id: 1 }, { id: 2 }], focusAttachment }, global: { stubs: { ApplicationAttachmentDetail: child } } });
    await flushPromises(); expect(focusAttachment).toHaveBeenCalledExactlyOnceWith({ id: 2 });
    await wrapper.setProps({ cars: [{ id: 3 }] }); await flushPromises();
    expect(focusAttachment).toHaveBeenCalledTimes(1);
    wrapper.unmount();
  });
});
