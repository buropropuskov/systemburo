import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
import UserApplications from '../UserApplications.vue';

function mountUA() {
  setActivePinia(createPinia());
  const replace = vi.fn(() => Promise.resolve());
  const wrapper = shallowMount(UserApplications, {
    props: { userId: 1 },
    global: { mocks: { $route: { query: {} }, $router: { replace, push: vi.fn() } } },
  });
  return { wrapper, replace };
}

describe('UserApplications — deep-link ?open (#973)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('не чистит query, пока заявка не появилась в списке (гонка userId)', () => {
    const { wrapper, replace } = mountUA();
    wrapper.vm.applications = [];
    wrapper.vm.$route.query = { open: '7' };
    wrapper.vm.openFromDeepLink();

    expect(replace).not.toHaveBeenCalled();
    expect(wrapper.vm.showDetailModal).toBe(false);
  });

  it('открывает заявку и чистит query, когда она есть в списке', async () => {
    const { wrapper, replace } = mountUA();
    wrapper.vm.applications = [{ id: 7, sender_user_id: 1, application_number: 'A-7' }];
    wrapper.vm.$route.query = { open: '7' };
    wrapper.vm.openFromDeepLink();
    await flushPromises();

    expect(wrapper.vm.selectedApplication).toEqual(expect.objectContaining({ id: 7 }));
    expect(wrapper.vm.showDetailModal).toBe(true);
    expect(replace).toHaveBeenCalled();
  });
});
