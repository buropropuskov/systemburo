import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils';

const mocks = vi.hoisted(() => ({ fetch: vi.fn(), collect: vi.fn(), download: vi.fn(), notify: vi.fn() }));
vi.mock('@/utils/systemTableHistory', () => ({ fetchSystemTableHistoryPage: mocks.fetch, collectSystemTableHistory: mocks.collect }));
vi.mock('@/utils/directoryHistoryExcel', () => ({ downloadDirectoryHistory: mocks.download }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: mocks.notify }) }));
import SystemTableHistoryModal from '../SystemTableHistoryModal.vue';

const item = id => ({ id, action_type: 'updated', details: {}, user_id: 4, user_name: 'Synthetic operator', created_at: '2030-01-02T10:00:00Z' });
const page = (items, total = items.length) => ({ items, total, page: 1, perPage: 50, snapshotMaxID: 90, nameMaskRevision: 'p1' });
function open() {
  return mount(SystemTableHistoryModal, {
    props: { table: { id: 7, display_name: 'Synthetic table' } },
    global: { stubs: { teleport: true, transition: false } },
  });
}
enableAutoUnmount(afterEach);

describe('SystemTableHistoryModal server pagination', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetch.mockReset().mockResolvedValue(page([item(1)], 2));
    mocks.collect.mockReset().mockResolvedValue([]);
    mocks.download.mockResolvedValue();
  });

  it('loads more with the original snapshot and offers actors from the full history', async () => {
    mocks.collect.mockResolvedValue([{ id: 9, name: 'Actor outside the visible page' }]);
    const wrapper = open();
    await flushPromises();
    expect(wrapper.text()).toContain('Показано 1 из 2');
    expect(wrapper.vm.uniqueUsers).toEqual([{ id: 9, name: 'Actor outside the visible page' }]);
    mocks.fetch.mockResolvedValueOnce(page([item(2)], 2));
    await wrapper.vm.loadMore();
    expect(mocks.fetch.mock.calls.at(-1)[2]).toMatchObject({ page: 2, snapshotMaxID: 90 });
    expect(wrapper.vm.history.map(row => row.id)).toEqual([1, 2]);
  });

  it('exports the separately collected full filtered dataset instead of visible rows', async () => {
    const wrapper = open();
    await flushPromises();
    mocks.collect.mockResolvedValueOnce([item(1), item(2)]);
    await wrapper.vm.exportToExcel();
    expect(mocks.download).toHaveBeenCalledTimes(1);
    expect(mocks.download.mock.calls[0][0].rows).toHaveLength(2);
  });

  it('does not create a partial workbook after a failed collection', async () => {
    const wrapper = open();
    await flushPromises();
    mocks.collect.mockRejectedValueOnce(new Error('privacy policy changed'));
    await wrapper.vm.exportToExcel();
    expect(mocks.download).not.toHaveBeenCalled();
    expect(mocks.notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.vm.isExporting).toBe(false);
  });

  it('ignores an older response after a newer filter has been applied', async () => {
    const wrapper = open();
    await flushPromises();
    let finishOld;
    mocks.fetch.mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve; }));
    const oldRequest = wrapper.vm.loadHistory();
    mocks.fetch.mockResolvedValueOnce(page([item(9)], 1));
    await wrapper.setData({ selectedUserId: 9 });
    await flushPromises();
    finishOld(page([item(3)], 1));
    await oldRequest;
    expect(wrapper.vm.history.map(row => row.id)).toEqual([9]);
    expect(mocks.fetch.mock.calls.at(-1)[1].userId).toBe(9);
  });
});
