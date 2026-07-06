import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { nextTick } from 'vue';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { useAuthStore } from '@/stores/auth';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));
// eventStream мокаем, чтобы его connect не дёргал apiRequest и не путал порядок промисов.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => () => {}),
    onStatus: vi.fn(() => () => {}),
  },
}));

const stubs = {
  teleport: true,
  OrganizationFilter: true,
  RefreshButton: true,
  ApplicationDetail: true,
  DateFilter: true,
  FilterTabs: true,
  SkeletonTransition: { template: '<div><slot /></div>' },
  SkeletonTable: true,
  LoaderSpinner: true,
  DownloadBlanksModal: true,
  Badge: true,
  BaseDropdown: true,
};

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn() } } },
  });
}

const resp = () => ({ ok: false, text: async () => '', json: async () => [] });

describe('ApplicationsCenter: оверлей refreshing и silent-режим', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('silent-обновление (SSE) не показывает оверлей refreshing', async () => {
    const resolvers = [];
    apiRequest.mockImplementation(() => new Promise((r) => resolvers.push(r)));
    useAuthStore().token = 'test-token';
    const w = mountCenter();
    await nextTick();
    // чистый старт после mounted-вызовов
    w.vm.refreshing = false;
    w.vm.pendingRefreshCount = 0;
    resolvers.length = 0;

    w.vm.fetchApplications(true); // silent
    await nextTick();
    expect(w.vm.refreshing).toBe(false);
  });

  it('не залипает при гонке silent(SSE) + non-silent(фильтр)', async () => {
    const resolvers = [];
    apiRequest.mockImplementation(() => new Promise((r) => resolvers.push(r)));
    useAuthStore().token = 'test-token';
    const w = mountCenter();
    await nextTick();
    w.vm.refreshing = false;
    w.vm.pendingRefreshCount = 0;
    resolvers.length = 0;

    w.vm.fetchApplications(); // non-silent (фильтр) - ставит оверлей, seq = N
    w.vm.fetchApplications(true); // silent (SSE) - новее по seq (N+1)
    await nextTick();
    expect(w.vm.refreshing).toBe(true);

    // silent резолвится ПЕРВЫМ - его seq актуален, но silent не трогает refreshing
    resolvers[1](resp());
    await nextTick();
    await nextTick();
    // non-silent резолвится позже: его seq устарел (N != N+1), но refreshing развязан
    // от seq (RED-фикс) - счётчик non-silent доходит до 0 и гасит оверлей.
    resolvers[0](resp());
    await nextTick();
    await nextTick();

    expect(w.vm.refreshing).toBe(false); // не залип
    expect(w.vm.pendingRefreshCount).toBe(0);
  });
});
