import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import BlacklistView from '../BlacklistView.vue';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => ({ username: 'admin' }) }),
}));

/**
 * Переход из сквозного поиска знает, человек нашёлся или машина. Без этого страница
 * всегда открывалась на вкладке машин, и найденного человека не было видно.
 */
function mountView(query) {
  setActivePinia(createPinia());
  return mount(BlacklistView, {
    global: {
      stubs: { FilterTabs: true, VehicleBlacklistTab: true, PersonBlacklistTab: true },
      mocks: { $route: { query }, $router: { replace: vi.fn() } },
    },
  });
}

describe('BlacklistView - вкладка из адреса', () => {
  it('tab=persons открывает вкладку людей', () => {
    expect(mountView({ tab: 'persons' }).vm.activeTab).toBe('persons');
  });

  it('tab=vehicles открывает вкладку машин', () => {
    expect(mountView({ tab: 'vehicles' }).vm.activeTab).toBe('vehicles');
  });

  it('без параметра остаётся прежнее поведение - машины', () => {
    expect(mountView({}).vm.activeTab).toBe('vehicles');
  });
});
