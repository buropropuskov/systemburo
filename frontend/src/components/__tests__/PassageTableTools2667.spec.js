import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import PassageTableTools from '../PassageTableTools.vue';
import PassageValidityCell from '../PassageValidityCell.vue';
import OpenPassagesModal from '../OpenPassagesModal.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { listOpenPassages } from '@/api/openPassages';
vi.mock('@/api/openPassages', () => ({ listOpenPassages: vi.fn(), closeOpenPassage: vi.fn(), revertPassageCorrection: vi.fn() }));
const wrappers = [];
const create = props => { const w = mount(PassageTableTools, { props, global: { stubs: { Teleport: true } } }); wrappers.push(w); return w; };
describe('shared passage table controls #2667', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.useFakeTimers({ toFake: ['Date', 'performance', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] }); vi.setSystemTime(new Date('2045-01-01T00:00:00Z'));
    listOpenPassages.mockReset().mockResolvedValue({ items: [], counts: { all_open: 0, attention: 0, unknown_time: 0 }, page: 1, per_page: 25, total: 0, server_now: '2035-01-01T12:00:00Z' }); });
  afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.useRealTimers(); });
  it.each([['car', 'Незакрытые проезды'], ['employee', 'Незакрытые проходы']])('opens a scoped %s list only by explicit click', async (kind, label) => {
    const w = create({ kind, tableID: 4 }); await flushPromises();
    expect(listOpenPassages).not.toHaveBeenCalled(); expect(w.find('button').text()).toBe(label);
    await w.find('[data-testid="open-passages-button"]').trigger('click'); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledWith(kind, 4, expect.objectContaining({ source: 'table' }));
    w.findComponent(OpenPassagesModal).vm.$emit('changed', {});
    expect(w.emitted('refresh')).toHaveLength(1);
    await w.setProps({ tableID: 5 }); await flushPromises();
    expect(w.findComponent(OpenPassagesModal).props('show')).toBe(false);
  });
  it('does not broaden missing table ID into admin scope', async () => {
    const w = create({ kind: 'car', tableID: null }); await flushPromises();
    expect(w.find('button').exists()).toBe(false); expect(listOpenPassages).not.toHaveBeenCalled();
  });
  it('makes administrative summary reachable only with managed right and explicit source', async () => {
    const w = create({ kind: 'employee', source: 'admin_summary' }); await flushPromises();
    expect(w.find('button').exists()).toBe(false);
    usePermissionsStore().$patch({ mode: 'normal', effective: { 'detail.passage.correct': { value: 'allow' } } }); await flushPromises();
    await w.find('[data-testid="open-passages-button"]').trigger('click'); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledWith('employee', null, expect.objectContaining({ source: 'admin_summary' }));
    usePermissionsStore().effective = {}; await flushPromises(); expect(w.find('button').exists()).toBe(false);
  });
  it('refreshes once at registered grace boundary using server time and stops after unmount', async () => {
    const onRefresh = vi.fn();
    const w = create({ kind: 'car', tableID: 4, onRefresh, rows: [{ server_now: '2035-01-01T12:00:00Z', passage_state: { open: false, grace_until: '2035-01-01T12:00:01Z' } }] });
    await vi.advanceTimersByTimeAsync(999); expect(w.emitted('refresh')).toBeUndefined();
    await vi.advanceTimersByTimeAsync(1); expect(w.emitted('refresh')).toHaveLength(1);
    expect(onRefresh).toHaveBeenCalledTimes(1);
    w.unmount(); expect(vi.getTimerCount()).toBe(0);
    await vi.advanceTimersByTimeAsync(30000); expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(listOpenPassages).not.toHaveBeenCalled();
  });
  it('renders compact frozen validity facts without timers or API', async () => {
    const w = mount(PassageValidityCell, { props: { item: { entry_date_to: '2035-01-01', admission: { reason: 'expired' }, passage_state: { open: true, entry_time_known: true } } } }); wrappers.push(w);
    expect(w.classes()).toContain('date-col'); expect(w.text()).toContain('01.01.2035');
    await w.find('button').trigger('click'); expect(w.find('[role="status"]').text()).toContain('Срок истёк');
    expect(vi.getTimerCount()).toBe(0); expect(listOpenPassages).not.toHaveBeenCalled();
  });
});
