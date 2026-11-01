import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia } from 'pinia';
import ApplicationHistory from '../ApplicationHistory.vue';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [
    { id: 1, user_id: 7, user_name: 'Соколов Алексей', action_type: 'created', created_at: '2026-01-10T10:00:00Z' },
    { id: 2, user_id: 8, user_name: 'Орлов Иван', action_type: 'approved', created_at: '2026-01-10T11:00:00Z' },
  ] }),
}));

describe('ApplicationHistory user filter', () => {
  let wrapper;
  let host;
  const dialog = () => document.querySelector('[data-testid="ob-history-modal"]');
  const dropdown = () => dialog()?.querySelector('.select-dropdown');
  const click = async (element) => {
    expect(element).not.toBeNull();
    element.click();
    await flushPromises();
  };

  async function open() {
    host = document.createElement('div');
    document.body.appendChild(host);
    wrapper = mount(ApplicationHistory, {
      attachTo: host,
      props: { applicationId: 1 },
      global: { plugins: [createPinia()], stubs: { AppIcon: true, ForwardHistoryDetails: true } },
    });
    await wrapper.get('.history-toggle').trigger('click');
    await flushPromises();
    await click(dialog().querySelector('.select-trigger'));
    expect(dropdown()).not.toBeNull();
  }

  afterEach(() => {
    wrapper?.unmount();
    host?.remove();
  });

  it('selects a user and closes the teleported dropdown', async () => {
    await open();
    await click(dropdown().querySelectorAll('.select-option')[1]);
    expect(wrapper.vm.selectedUserId).not.toBeNull();
    expect(dialog().querySelectorAll('.history-item')).toHaveLength(1);
    expect(dropdown()).toBeNull();
    expect(wrapper.vm.showModal).toBe(true);
  });

  it('closes the dropdown on a click inside the dialog but outside the select', async () => {
    await open();
    await click(dialog().querySelector('h3'));
    expect(dropdown()).toBeNull();
    expect(wrapper.vm.showModal).toBe(true);
  });

  it('clears a selected user and closes the all-users option', async () => {
    await open();
    await wrapper.setData({ selectedUserId: 7 });
    await click(dropdown().querySelector('.select-option'));
    expect(wrapper.vm.selectedUserId).toBeNull();
    expect(dialog().querySelectorAll('.history-item')).toHaveLength(2);
    expect(dropdown()).toBeNull();
  });
});
