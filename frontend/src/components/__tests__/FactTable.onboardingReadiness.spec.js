import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
const request = vi.hoisted(() => vi.fn());
vi.mock('@/api/client', () => ({ apiRequest: request }));
vi.mock('@/services/eventStream', () => ({ default: { connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => vi.fn()), onStatus: vi.fn(() => vi.fn()) } }));
import FactTable from '../FactTable.vue';
import FactTableTitle from '../FactTableTitle.vue';
import { normalizeFactTableRows } from '../factTableRows';
const ok = (data) => ({ ok: true, json: async () => data });
const deferred = () => { let resolve; const promise = new Promise((r) => { resolve = r; }); return { promise, resolve }; };
function mountTable() { return shallowMount(FactTable, { props: { tableType: 'cars', tableId: 42 }, global: { stubs: { 'transition-group': false } } }); }
beforeEach(() => { setActivePinia(createPinia()); request.mockReset(); request.mockResolvedValue(ok([])); });
describe('FactTable onboarding owner', () => {
  it('retains the title hierarchy and both entity labels after extracting scoped content', async () => {
    const w = mount(FactTableTitle, { props: { tableType: 'cars' } });
    expect(w.element.tagName).toBe('DIV');
    expect(w.classes()).toContain('card-header__title');
    expect(w.get('h3.card-title').text()).toBe('Автомобили по факту');
    expect(w.get('h3 > span.highlight-text').text()).toBe('по факту');
    await w.setProps({ tableType: 'people' });
    expect(w.get('h3.card-title').text()).toBe('Люди по факту');
    w.unmount();
  });
  it('retains row identities, organization lookup, detail aliases and passage precedence', () => {
    const source = { id: 7, organization: 'Организация', organization_id: 99,
      application_id: 12, car_number: 'А123ВС77', car_brand: 'ГАЗ', unload_place_ids: [4], territory_status: 2 };
    const passageFields = vi.fn(car => ({ exit_checked: car.territory_status === 2 }));
    const [row] = normalizeFactTableRows([source], { 3: 'Организация' }, passageFields);
    expect(row).toMatchObject({ id: 7, organization_id: '3', applicationId: 12,
      car_number: 'А123ВС77', plateNumber: 'А123ВС77', mark: 'ГАЗ', unloadPlaces: [4],
      entry_checked: false, exit_checked: true, entry_time_from: '', status: 'В работе' });
    expect(passageFields).toHaveBeenCalledWith(source);
    expect(source).not.toHaveProperty('exit_checked');
    const [empty] = normalizeFactTableRows([{ id: 9, organization_id: 6 }], {}, () => ({}));
    expect(empty).toMatchObject({ id: 9, organization_id: 6, car_number: 'по факту',
      organization_name: 'Не указана', unload_place: '-', company: null });
  });
  it('distinguishes successful empty and failed/forbidden list', async () => {
    const w = mountTable(); await flushPromises();
    expect(w.attributes('data-ob-owner')).toBe('fact-list');
    expect(w.attributes('data-ob-state')).toBe('empty');
    request.mockImplementation(async (url) => url.startsWith('/cars/fact-for-table/') ? { ok: false, status: 500 } : ok([]));
    await w.vm.silentRefresh(); await flushPromises(); expect(w.attributes('data-ob-state')).toBe('error');
    request.mockImplementation(async (url) => url.startsWith('/cars/fact-for-table/') ? { ok: false, status: 403 } : ok([]));
    await w.vm.silentRefresh(); await flushPromises(); expect(w.attributes('data-ob-state')).toBe('forbidden'); w.unmount();
  });
  it('does not let stale success release newer pending owner and pins row ID', async () => {
    const w = mountTable(); await flushPromises();
    const first = deferred(); const second = deferred(); let calls = 0;
    request.mockImplementation((url) => url.startsWith('/cars/fact-for-table/') ? (++calls === 1 ? first.promise : second.promise) : Promise.resolve(ok([])));
    const a = w.vm.silentRefresh(); await flushPromises();
    const b = w.vm.silentRefresh(); await flushPromises();
    first.resolve(ok([{ id: 11 }])); await a; await flushPromises();
    expect(w.attributes('data-ob-state')).toBe('loading');
    second.resolve(ok([{ id: 22 }])); await b; await flushPromises();
    expect(w.attributes('data-ob-state')).toBe('ready');
    expect(w.find('[data-testid="ob-fact-row"]').attributes('data-ob-row-key')).toBe('22'); w.unmount();
  });
  it('does not turn independent options failure into list failure', async () => {
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {});
    request.mockImplementation(async (url) => {
      if (url.startsWith('/organizations')) throw new Error('options');
      return ok(url.startsWith('/cars/fact-for-table/') ? [{ id: 7 }] : []);
    });
    const w = mountTable(); await flushPromises();
    expect(w.attributes('data-ob-state')).toBe('ready');
    w.unmount(); errorLog.mockRestore();
  });
});
