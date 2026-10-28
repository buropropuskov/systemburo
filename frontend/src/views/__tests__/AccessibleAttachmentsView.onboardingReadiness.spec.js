import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
const api = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn(), orgs: vi.fn(), companies: vi.fn() }));
vi.mock('@/api/applications', () => ({ getAccessibleAttachments: api.list, getAccessibleAttachmentDetail: api.detail, markAccessibleAttachmentExecuted: vi.fn() }));
vi.mock('@/api/organizations', () => ({ getOrganizations: api.orgs, getCompanies: api.companies }));
vi.mock('@/api/attachment-templates', () => ({ previewBlank: vi.fn() }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: vi.fn() }) }));
vi.mock('@/services/eventStream', () => ({ default: { connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => vi.fn()) } }));
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ replace: vi.fn(() => Promise.resolve()) }), createRouter: () => ({ beforeEach: vi.fn(), afterEach: vi.fn(), push: vi.fn() }), createWebHistory: () => ({}) }));
import View from '../AccessibleAttachmentsView.vue';
import AccessibleAttachmentsEmptyState from '@/components/AccessibleAttachmentsEmptyState.vue';
const deferred = () => { let resolve; const promise = new Promise((r) => { resolve = r; }); return { promise, resolve }; };
const result = (items = []) => ({ items, meta: { total: items.length } });
const item = { attachment_id: 9, attachment_type: 'cars', application_number: '20261028/001' };
function mountView() { return shallowMount(View, { global: { stubs: { AdminPageShell: { template: '<div><slot /></div>' } } } }); }
const state = (w, owner) => w.find('[data-ob-owner="accessible-' + owner + '"]').attributes('data-ob-state');
beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks(); api.list.mockResolvedValue(result()); api.orgs.mockResolvedValue([]); api.companies.mockResolvedValue([]); });
describe('accessible attachment owners', () => {
  it('retries the failed list through its owned refresh handler', async () => {
    api.list.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(result());
    const w = mountView(); await flushPromises(); expect(state(w, 'list')).toBe('error');
    const event = new CustomEvent('onboarding-retry', { cancelable: true });
    w.get('[data-ob-owner="accessible-list"]').element.dispatchEvent(event);
    await flushPromises();
    expect(event.defaultPrevented).toBe(true); expect(api.list).toHaveBeenCalledTimes(2);
    expect(state(w, 'list')).toBe('empty'); w.unmount();
  });
  it('retries the same failed attachment after the failed panel was hidden', async () => {
    api.list.mockResolvedValue(result([item]));
    api.detail.mockRejectedValueOnce(Object.assign(new Error('denied'), { status: 403 }))
      .mockResolvedValueOnce({ attachment: item, cars: [] });
    const w = mountView(); await flushPromises(); await w.get('[data-testid="aa-card"]').trigger('click'); await flushPromises();
    expect(state(w, 'detail')).toBe('forbidden');
    w.get('[data-ob-owner="accessible-detail"]').element.dispatchEvent(new CustomEvent('onboarding-retry', { cancelable: true }));
    await flushPromises();
    expect(api.detail).toHaveBeenNthCalledWith(2, item.attachment_id);
    expect(state(w, 'detail')).toBe('ready'); w.unmount();
  });
  it('retains the empty-state markup, messages and reset event after extraction', async () => {
    const w = mount(AccessibleAttachmentsEmptyState, { props: { hasActiveFilters: false } });
    expect(w.element.tagName).toBe('DIV');
    expect(w.attributes('data-testid')).toBe('aa-empty');
    expect(w.classes()).toContain('empty-state');
    expect(w.get('p').text()).toBe('Доступных вложений нет');
    expect(w.get('.empty-state__hint').text()).toContain('подтверждённых заявок');
    expect(w.find('[data-testid="aa-empty-reset"]').exists()).toBe(false);
    await w.setProps({ hasActiveFilters: true });
    expect(w.get('p').text()).toBe('Ничего не найдено');
    await w.get('[data-testid="aa-empty-reset"]').trigger('click');
    expect(w.emitted('reset')).toEqual([[]]);
    w.unmount();
  });
  it('distinguishes list error without blocking static filters', async () => {
    api.list.mockRejectedValue(new Error('network'));
    const w = mountView(); await flushPromises(); expect(state(w, 'list')).toBe('error');
    expect(w.find('[data-testid="aa-filters"]').element.closest('[data-ob-owner="accessible-list"]')).toBe(null); w.unmount();
  });
  it('does not turn independent options failure into list failure', async () => {
    api.orgs.mockRejectedValue(new Error('options')); const w = mountView(); await flushPromises(); expect(state(w, 'list')).toBe('empty'); w.unmount();
  });
  it('does not publish an old list success during a newer request', async () => {
    const a = deferred(); const b = deferred();
    api.list.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise);
    const w = mountView(); await flushPromises();
    w.findComponent({ name: 'RefreshButton' }).vm.$emit('refresh'); await flushPromises();
    a.resolve(result([item])); await flushPromises();
    expect(state(w, 'list')).toBe('loading');
    b.resolve(result()); await flushPromises();
    expect(state(w, 'list')).toBe('empty'); w.unmount();
  });
  it('keeps a detail failure observable after failed opening hides the panel', async () => {
    api.list.mockResolvedValue(result([item])); api.detail.mockRejectedValue(Object.assign(new Error('denied'), { status: 403 }));
    const w = mountView(); await flushPromises(); await w.find('[data-testid="aa-card"]').trigger('click'); await flushPromises();
    expect(state(w, 'detail')).toBe('forbidden'); expect(w.find('[data-testid="aa-detail"]').exists()).toBe(false); w.unmount();
  });
  it('prevents stale detail success reopening a cleared selection', async () => {
    const d = deferred(); api.list.mockResolvedValue(result([item])); api.detail.mockReturnValue(d.promise);
    const w = mountView(); await flushPromises(); await w.find('[data-testid="aa-card"]').trigger('click'); await flushPromises(); expect(state(w, 'detail')).toBe('loading');
    w.findComponent({ name: 'RefreshButton' }).vm.$emit('refresh'); await flushPromises();
    d.resolve({ attachment: item }); await flushPromises(); expect(state(w, 'detail')).toBe('empty'); expect(w.find('[data-testid="aa-detail"]').exists()).toBe(false); w.unmount();
  });
});
