import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import OpenPassagesModal from '../OpenPassagesModal.vue';
import BaseDropdown from '../ui/BaseDropdown.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { listOpenPassages, closeOpenPassage, revertPassageCorrection } from '@/api/openPassages';

vi.mock('@/api/openPassages', () => ({ listOpenPassages: vi.fn(), closeOpenPassage: vi.fn(), revertPassageCorrection: vi.fn() }));
const wrappers = [];
const stamp = '2035-01-03T12:00:00Z';
const passage = { has_event: true, last_event_id: 81, open: true, entry_at: '2035-01-01T11:00:00Z', entry_table_id: 4,
  exit_recorded_at: null, grace_until: null, in_exit_grace: false, needs_attention: true, entry_time_known: true,
  last_event_kind: 'entry', can_correct: true, can_revert_correction: false };
const row = { entity_id: 17, entity_kind: 'car', attachment_id: 8, application_id: null, application_number: null,
  organization_id: 7, organization: 'Synthetic organization', display_name: 'TEST-17', server_now: stamp,
  effective_period: { bounded: true, entry_date_from: '2034-12-01', entry_date_to: '2035-01-02', entry_time_from: '08:00:00', entry_time_to: '20:00:00' },
  admission: { can_enter: false, can_exit: true, reason: 'expired' }, passage_state: passage };
const result = (items = [row]) => ({ items, counts: { all_open: 3, attention: 1, unknown_time: 1 }, page: 1, per_page: 25, total: items.length, server_now: stamp });
function create(props = {}) {
  const wrapper = mount(OpenPassagesModal, { props: { show: true, kind: 'car', tableID: 4, organizations: [{ id: 7, name: 'Synthetic organization' }], ...props },
    attachTo: document.body, global: { stubs: { Teleport: true } } });
  wrappers.push(wrapper); return wrapper;
}
async function selectMode(wrapper, mode) {
  wrapper.findAllComponents(BaseDropdown).find(d => d.attributes('data-testid') === 'open-passages-mode').vm.$emit('update:modelValue', mode);
  await flushPromises();
}
async function openCorrection(wrapper, revert = false) {
  await flushPromises();
  const label = revert ? 'Отменить исправление' : 'Исправить учёт';
  await wrapper.findAll('button').find(button => button.text() === label).trigger('click');
  await wrapper.find('[data-testid="passage-correction-reason"]').setValue('Synthetic correction reason');
}
describe('open passage real modal #2667', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date', 'performance', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] });
    vi.setSystemTime(new Date('2045-01-01T00:00:00Z'));
    setActivePinia(createPinia());
    usePermissionsStore().$patch({ mode: 'normal', effective: { 'detail.passage.correct': { value: 'allow' } } });
    listOpenPassages.mockReset().mockResolvedValue(result()); closeOpenPassage.mockReset(); revertPassageCorrection.mockReset();
  });
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers(); });
  it('loads current table attention by default using real project controls without autofocus', async () => {
    const wrapper = create(); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledWith('car', 4, { source: 'table', attentionOnly: true, view: 'open', search: '', organizationID: null, page: 1, perPage: 25 });
    expect(wrapper.text()).toContain('Незакрытых: 3');
    expect(wrapper.text()).toContain('49 ч 0 мин');
    expect(wrapper.find('[autofocus]').exists()).toBe(false);
    expect(wrapper.findAllComponents(BaseDropdown).every(dropdown => !dropdown.vm.isOpen)).toBe(true);
    expect(document.activeElement.tagName).toBe('BODY');
  });
  it('uses all/open and corrections as distinct server-listed filters, with no local correction rows', async () => {
    const wrapper = create(); await flushPromises();
    await selectMode(wrapper, 'all');
    expect(listOpenPassages).toHaveBeenLastCalledWith('car', 4, expect.objectContaining({ view: 'open', attentionOnly: false }));
    listOpenPassages.mockResolvedValueOnce(result([]));
    await selectMode(wrapper, 'corrections');
    expect(listOpenPassages).toHaveBeenLastCalledWith('car', 4, expect.objectContaining({ view: 'corrections' }));
    expect(wrapper.find('[data-testid="open-passage-17"]').exists()).toBe(false);
    await wrapper.setProps({ show: false }); await wrapper.setProps({ show: true }); await flushPromises();
    expect(listOpenPassages).toHaveBeenLastCalledWith('car', 4, expect.objectContaining({ view: 'open', attentionOnly: true }));
  });
  it('does not expose correction controls without the managed permission', async () => {
    usePermissionsStore().effective = {};
    const wrapper = create(); await flushPromises();
    expect(wrapper.findAll('button').some(button => button.text() === 'Исправить учёт')).toBe(false);
    const options = wrapper.findAllComponents(BaseDropdown)[0].props('options');
    expect(options.some(option => option.id === 'corrections')).toBe(false);
  });
  it('preserves explicit unknown time, sends expected event once, and reloads after correction', async () => {
    const wrapper = create(); await openCorrection(wrapper);
    expect(wrapper.text()).toContain('неизвестное');
    let resolve;
    closeOpenPassage.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click');
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click');
    expect(closeOpenPassage).toHaveBeenCalledTimes(1);
    expect(closeOpenPassage).toHaveBeenCalledWith('car', 17, 4, { source: 'table', expected_last_event_id: 81, reason: 'Synthetic correction reason', actual_exit_at: null });
    listOpenPassages.mockResolvedValueOnce(result([]));
    resolve({ ...row, passage_state: { ...passage, last_event_id: 82, open: false } }); await flushPromises();
    expect(wrapper.emitted('changed')).toHaveLength(1);
    expect(listOpenPassages).toHaveBeenCalledTimes(2);
    expect(wrapper.text()).toContain('Учёт исправлен');
  });
  it('converts known Moscow time explicitly and checks it against server time rather than client time', async () => {
    const wrapper = create(); await openCorrection(wrapper);
    await wrapper.find('input[type="checkbox"]').setValue(true);
    const actual = wrapper.find('[data-testid="passage-correction-actual"]');
    await actual.setValue('2035-01-03T16:00');
    expect(wrapper.find('[data-testid="passage-correction-save"]').attributes('disabled')).toBeDefined();
    await actual.setValue('2035-01-03T14:30');
    closeOpenPassage.mockResolvedValueOnce(row);
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click'); await flushPromises();
    expect(closeOpenPassage).toHaveBeenCalledWith('car', 17, 4, expect.objectContaining({ actual_exit_at: '2035-01-03T11:30:00.000Z' }));
  });
  it('requires refresh after conflict, never retries the correction automatically', async () => {
    const wrapper = create(); await openCorrection(wrapper);
    closeOpenPassage.mockRejectedValueOnce(Object.assign(new Error('Changed'), { status: 409 }));
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click'); await flushPromises();
    expect(wrapper.text()).toContain('автоматического повтора нет');
    expect(wrapper.find('[data-testid="passage-correction-save"]').attributes('disabled')).toBeDefined();
    expect(closeOpenPassage).toHaveBeenCalledTimes(1);
    await wrapper.findAll('button').find(button => button.text() === 'Обновить').trigger('click'); await flushPromises();
    expect(wrapper.find('[data-testid="passage-correction-form"]').exists()).toBe(false);
    expect(closeOpenPassage).toHaveBeenCalledTimes(1);
  });
  it('server-lists a reversible recent correction and hides its action after the exact 15-minute window', async () => {
    const correction = { ...row, passage_state: { ...passage, open: false, last_event_kind: 'correction',
      can_correct: false, can_revert_correction: true, exit_recorded_at: '2035-01-03T11:45:00Z' } };
    const wrapper = create(); await flushPromises();
    listOpenPassages.mockResolvedValue(result([correction]));
    await selectMode(wrapper, 'corrections'); await openCorrection(wrapper, true);
    expect(wrapper.find('[data-testid="passage-correction-save"]').attributes('disabled')).toBeUndefined();
    vi.advanceTimersByTime(1); await flushPromises();
    expect(wrapper.find('[data-testid="passage-correction-save"]').attributes('disabled')).toBeDefined();
    expect(revertPassageCorrection).not.toHaveBeenCalled();
  });
  it('reverts only a server-listed correction with its current event ID and an explicit reason', async () => {
    const correction = { ...row, passage_state: { ...passage, last_event_id: 82, open: false, last_event_kind: 'correction',
      can_correct: false, can_revert_correction: true, exit_recorded_at: stamp } };
    const wrapper = create(); await flushPromises();
    listOpenPassages.mockResolvedValue(result([correction]));
    await selectMode(wrapper, 'corrections'); await openCorrection(wrapper, true);
    revertPassageCorrection.mockResolvedValueOnce(row);
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click'); await flushPromises();
    expect(revertPassageCorrection).toHaveBeenCalledWith('car', 17, 4, { source: 'table', expected_last_event_id: 82, reason: 'Synthetic correction reason' });
    expect(closeOpenPassage).not.toHaveBeenCalled();
    expect(wrapper.emitted('changed')).toEqual([[row]]);
  });
  it('ignores late reads and late saves when table context changes', async () => {
    let resolveRead;
    listOpenPassages.mockReturnValueOnce(new Promise(done => { resolveRead = done; }));
    const wrapper = create();
    listOpenPassages.mockResolvedValueOnce(result([]));
    await wrapper.setProps({ tableID: 9 }); await flushPromises();
    resolveRead(result()); await flushPromises();
    expect(wrapper.find('[data-testid="open-passage-17"]').exists()).toBe(false);
    await wrapper.setProps({ tableID: 4 }); await openCorrection(wrapper);
    let resolveSave;
    closeOpenPassage.mockReturnValueOnce(new Promise(done => { resolveSave = done; }));
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click');
    await wrapper.setProps({ tableID: 9 }); await flushPromises();
    resolveSave(row); await flushPromises();
    expect(wrapper.emitted('changed')).toBeUndefined();
  });
  it('keeps unknown entry time explicit without inventing a duration', async () => {
    listOpenPassages.mockResolvedValueOnce(result([{ ...row, passage_state: { ...passage, entry_at: null, entry_time_known: false, needs_attention: false } }]));
    const wrapper = create(); await flushPromises();
    expect(wrapper.text()).toContain('Вход: время неизвестно');
    expect(wrapper.text()).toContain('Длительность: неизвестна');
  });
  it('formats ISO instants as Moscow dates and renders null or malformed instants safely', async () => {
    const wrapper = create(); await flushPromises();
    expect(wrapper.text()).toContain('01.01.2035 14:00:00');
    listOpenPassages.mockResolvedValueOnce(result([{ ...row, passage_state: { ...passage,
      entry_at: 'not-an-instant', exit_recorded_at: null } }]));
    await selectMode(wrapper, 'corrections');
    expect(wrapper.text()).toContain('Вход: неизвестно');
    expect(wrapper.text()).toContain('Исправление зарегистрировано: неизвестно');
  });
  it('refreshes an all-open list strictly after its known entry reaches 48 hours', async () => {
    listOpenPassages.mockResolvedValueOnce(result([]));
    const wrapper = create(); await flushPromises();
    listOpenPassages.mockResolvedValue(result([{ ...row, passage_state: { ...passage,
      entry_at: '2035-01-01T12:00:01Z', needs_attention: false } }]));
    await selectMode(wrapper, 'all');
    const calls = listOpenPassages.mock.calls.length;
    vi.advanceTimersByTime(1000); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(calls);
    vi.advanceTimersByTime(1); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(calls + 1);
  });
  it('requires explicit reselection when a background refresh replaces event 81 with 83', async () => {
    const wrapper = create(); await openCorrection(wrapper);
    await wrapper.find('input[type="checkbox"]').setValue(true);
    await wrapper.find('[data-testid="passage-correction-actual"]').setValue('2035-01-03T14:30');
    listOpenPassages.mockResolvedValue(result([{ ...row, passage_state: { ...passage, last_event_id: 83 } }]));
    vi.advanceTimersByTime(30000); await flushPromises();
    expect(wrapper.find('[data-testid="passage-correction-form"]').exists()).toBe(false);
    expect(wrapper.text()).toContain('Последняя отметка изменилась');
    expect(closeOpenPassage).not.toHaveBeenCalled();
    await wrapper.findAll('button').find(button => button.text() === 'Исправить учёт').trigger('click');
    expect(wrapper.find('[data-testid="passage-correction-reason"]').element.value).toBe('');
    expect(wrapper.find('input[type="checkbox"]').element.checked).toBe(false);
    await wrapper.find('[data-testid="passage-correction-reason"]').setValue('New reviewed reason');
    closeOpenPassage.mockResolvedValueOnce(row);
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click'); await flushPromises();
    expect(closeOpenPassage).toHaveBeenCalledWith('car', 17, 4, { source: 'table', expected_last_event_id: 83, reason: 'New reviewed reason', actual_exit_at: null });
  });
  it('refreshes an initially empty attention list/count within 30 seconds, and stops while hidden', async () => {
    listOpenPassages.mockResolvedValueOnce({ ...result([]), counts: { all_open: 1, attention: 0, unknown_time: 0 } });
    const wrapper = create(); await flushPromises();
    expect(wrapper.find('[data-testid="open-passage-17"]').exists()).toBe(false);
    vi.advanceTimersByTime(29999); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(2);
    expect(wrapper.find('[data-testid="open-passage-17"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="open-passages-counts"]').text()).toContain('более 48 часов: 1');
    await wrapper.setProps({ show: false });
    vi.advanceTimersByTime(60000); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(2);
  });
  it('does not overlap polling with a pending read and releases the poll on unmount', async () => {
    let resolve;
    listOpenPassages.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    const wrapper = create();
    vi.advanceTimersByTime(90000); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(1);
    resolve(result([])); await flushPromises();
    wrapper.unmount(); wrappers.splice(wrappers.indexOf(wrapper), 1);
    vi.advanceTimersByTime(60000); await flushPromises();
    expect(listOpenPassages).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });
  it('keeps explicit administrative scope throughout list and correction, and clears it on context change', async () => {
    const wrapper = create({ source: 'admin_summary', tableID: null }); await openCorrection(wrapper);
    expect(listOpenPassages).toHaveBeenCalledWith('car', null, expect.objectContaining({ source: 'admin_summary' }));
    closeOpenPassage.mockResolvedValueOnce({ ...row, passage_state: { ...passage, open: false, last_event_id: 82 } });
    await wrapper.find('[data-testid="passage-correction-save"]').trigger('click'); await flushPromises();
    expect(closeOpenPassage).toHaveBeenCalledWith('car', 17, null, expect.objectContaining({ source: 'admin_summary', expected_last_event_id: 81 }));
    await wrapper.setProps({ source: 'table', tableID: 4 }); await flushPromises();
    expect(wrapper.find('[data-testid="passage-correction-form"]').exists()).toBe(false);
    expect(listOpenPassages).toHaveBeenLastCalledWith('car', 4, expect.objectContaining({ source: 'table' }));
  });
});
