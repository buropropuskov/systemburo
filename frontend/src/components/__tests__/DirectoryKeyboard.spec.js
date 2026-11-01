import { beforeEach, afterEach, describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async () => ({ ok: false, json: async () => [] })) }));
vi.mock('@/api/organizations', () => ({ getCompanyMembers: vi.fn(async () => []), getOrganizationMembers: vi.fn(async () => []) }));
vi.mock('@/utils/dirtyTracker', () => ({ registerDirtyTracker: () => () => {}, confirmIfAnyDirty: vi.fn(async () => true) }));
import Companies from '../CompaniesManagement.vue';
import Organizations from '../OrganizationsManagement.vue';
import { useCompaniesStore } from '@/stores/companies';
import { useOrganizationsStore } from '@/stores/organizations';
const wrappers = [];
beforeEach(() => setActivePinia(createPinia()));
afterEach(() => { wrappers.forEach(w => w.unmount()); wrappers.length = 0; vi.restoreAllMocks(); });
const cases = [
  { name: 'companies', Component: Companies, store: useCompaniesStore, selector: 'companies-row', selected: 'selectedCompany', details: 'companies-detail-name' },
  { name: 'organizations', Component: Organizations, store: useOrganizationsStore, selector: 'orgs-row', selected: 'selectedOrganization', details: 'orgs-detail-name' },
];
describe.each(cases)('Native actions in $name', ({ Component, store: createStore, selector, selected }) => {
  async function setup() {
    const store = createStore();
    store.itemsWithUsers = [{ id: 1, name: 'Альфа', is_active: true, user_count: 0 }, { id: 2, name: 'Бета', is_active: true, user_count: 1 }];
    vi.spyOn(store, 'refresh').mockResolvedValue();
    const w = mount(Component, { global: { stubs: { ResponsibleUsersSection: true, SelectUnloadPlaces: true, SelectTables: true, DirectoryModeration: true } } });
    wrappers.push(w);
    await flushPromises();
    return w;
  }
  it('opens through the name button and leaves checkbox selection independent', async () => {
    const w = await setup();
    const row = w.get('[data-testid="' + selector + '"]');
    const open = row.get('.directory-record-open');
    expect(open.element.tagName).toBe('BUTTON');
    expect(open.attributes('type')).toBe('button');
    expect(open.find('button,input').exists()).toBe(false);
    await row.get('input[type=checkbox]').setValue(true);
    expect(w.vm[selected]).toBeNull();
    await open.trigger('click');
    expect(w.vm[selected].id).toBe(1);
  });
  it('sorts through the shared header button in both directions', async () => {
    const w = await setup();
    const sort = w.get('button[data-field=name]');
    await sort.trigger('click');
    expect(w.vm.sortField).toBe('name');
    const direction = w.vm.sortDirection;
    await sort.trigger('click');
    expect(w.vm.sortDirection).not.toBe(direction);
    expect(sort.attributes('aria-label')).toContain(w.vm.sortDirection === 'asc' ? 'по возрастанию' : 'по убыванию');
  });
});
