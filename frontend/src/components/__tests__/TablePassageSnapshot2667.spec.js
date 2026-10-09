import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CarsTable from '../CarsTable.vue';
import PeopleTable from '../PeopleTable.vue';
import { apiRequest } from '@/api/client';
import { normalizeSnapshotRows } from '@/utils/snapshotRows';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/services/eventStream', () => ({ default: { connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => vi.fn()), onStatus: vi.fn(() => vi.fn()) } }));
const wrappers = [];
describe('real table frozen passage rendering #2667', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.useFakeTimers(); apiRequest.mockReset(); });
  afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.useRealTimers(); });
  it.each([[CarsTable, 'cars'], [PeopleTable, 'people']])('%s snapshot has frozen facts and no live controls, requests or clocks', async (component, kind) => {
    const rows = normalizeSnapshotRows([{ id: 17, car_number: 'TEST-17', last_name: 'Synthetic', entry_date_to: '2035-01-01',
      server_now: '2035-01-03T12:00:00Z', admission: { can_enter: false, can_exit: true, reason: 'expired' },
      effective_period: { bounded: true, source: 'individual' }, passage_state: { open: true, entry_time_known: true,
        last_event_kind: 'entry', last_event_id: 81, entry_at: '2035-01-01T11:00:00Z', needs_attention: true, can_correct: true } }], kind);
    const w = mount(component, { props: { preview: true, previewItems: rows, previewFields: [
      { field_name: kind === 'cars' ? 'car_number' : 'last_name', is_visible: true }, { field_name: 'valid_until', is_visible: true }], tableName: 'Synthetic post' },
      global: { stubs: { Teleport: true, transition: false, 'transition-group': false } } }); wrappers.push(w); await flushPromises();
    expect(apiRequest).not.toHaveBeenCalled();
    expect(w.find('[data-testid="open-passages-button"]').exists()).toBe(false);
    // PeopleTable has no tour test IDs on these buttons; both actual tables
    // expose the established row action classes and must remain read-only.
    const entryButtons = w.findAll('.rt-row .entry-btn'), exitButtons = w.findAll('.rt-row .exit-btn');
    expect(entryButtons).toHaveLength(1); expect(exitButtons).toHaveLength(1);
    expect(entryButtons[0].attributes()).toHaveProperty('disabled');
    expect(exitButtons[0].attributes()).toHaveProperty('disabled');
    await entryButtons[0].trigger('click'); await exitButtons[0].trigger('click');
    expect(apiRequest).not.toHaveBeenCalled();
    expect(w.vm.itemsData[0].passage_state.last_event_id).toBe(81);
    const timers = vi.getTimerCount(); await vi.advanceTimersByTimeAsync(3600000);
    expect(w.vm.itemsData[0].passage_state.needs_attention).toBe(true);
    expect(apiRequest).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBeLessThanOrEqual(timers);
  });
});
