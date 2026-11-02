import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import TrashView from '../TrashView.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async path => ({ ok: true, json: async () => path.startsWith('/system-tables/name/') ? { table: { id: 42, name: 'alpha', table_type: 'cars' } } : path === '/users/me' ? { username: 'alpha' } : [] })) }));
vi.mock('@/api/trash', () => ({ listTrash: vi.fn(async () => []), restoreItems: vi.fn(), purgeItem: vi.fn(), clearTrash: vi.fn() }));
const wrappers = [];
beforeEach(() => setActivePinia(createPinia()));
afterEach(() => { wrappers.forEach(w => w.unmount()); wrappers.length = 0; vi.restoreAllMocks(); });
async function setup(type, filled = true) {
  const w = mount(TrashView, { global: { mocks: { $route: { params: { tableName: 'alpha' } } }, stubs: { RouterLink: true, VehicleDetailsModal: true, EmployeeDetailsModal: true, ApplicationDetail: true, TrashHistoryModal: true, ConfirmationModal: true, DateFilter: true } } });
  wrappers.push(w); await flushPromises();
  await w.setData({ tableType: type, items: filled ? [
    { id: 1, organization: 'Альфа', car_number: 'А111АА77', last_name: 'Альфа', deleted_at: '2026-01-01T10:00:00Z' },
    { id: 2, organization: 'Бета', car_number: 'В222ВВ77', last_name: 'Бета', deleted_at: '2026-01-02T10:00:00Z' },
  ] : [] });
  return w;
}
describe.each(['cars', 'people'])('Trash sorting for %s', type => {
  it('sorts once per native button activation and preserves selection', async () => {
    const w = await setup(type), open = vi.spyOn(w.vm, 'onRowClick');
    const button = w.get('button[data-field="organization"]'), header = button.element.closest('th');
    expect(button.element.type).toBe('button');
    expect(header.getAttribute('aria-sort')).toBe('none');
    await w.get('[data-testid="trash-row-check"]').setValue(true);
    const selected = [...w.vm.selectedIds];
    await button.trigger('click');
    expect(w.vm.sortDir).toBe('desc');
    expect(header.getAttribute('aria-sort')).toBe('descending');
    expect(w.vm.sortedItems.map(i => i.id)).toEqual([2, 1]);
    await button.trigger('click');
    expect(w.vm.sortDir).toBe('asc');
    expect(header.getAttribute('aria-sort')).toBe('ascending');
    expect(w.vm.sortedItems.map(i => i.id)).toEqual([1, 2]);
    expect(w.vm.selectedIds).toEqual(selected);
    expect(open).not.toHaveBeenCalled();
  });
  it('keeps mouse activation on header padding and excludes non-sortable columns', async () => {
    const w = await setup(type), button = w.get('button[data-field="organization"]');
    await w.get('th:has(button[data-field="organization"])').trigger('click');
    expect(w.vm.sortField).toBe('organization');
    expect(w.vm.sortDir).toBe('desc');
    await button.trigger('click');
    expect(w.vm.sortDir).toBe('asc');
    expect(w.find('button[data-field="time"]').exists()).toBe(false);
    expect(w.find('button[data-field="status"]').exists()).toBe(false);
  });
});
it('keeps the empty state without table controls', async () => {
  const w = await setup('cars', false);
  expect(w.text()).toContain('Корзина пуста');
  expect(w.find('button[data-field]').exists()).toBe(false);
});
