import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { nextTick } from 'vue';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
// fetchApplications (список Центра, #1158) идёт через getApplicationsPaginated,
// не через apiRequest напрямую - мокаем отдельно, чтобы контролировать момент резолва.
vi.mock('@/api/applications', () => ({ getApplicationsPaginated: vi.fn() }));
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
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

const page = (items = []) => ({ items, meta: { total: items.length, page: 1, per_page: 30 } });

describe('ApplicationsCenter: оверлей refreshing и silent-режим', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    // fetchOrganizations/getCurrentUser (тоже дёргаются в mounted) остаются на apiRequest -
    // этот тест их не проверяет, просто резолвим сразу, чтобы не шуметь неотловленными промисами.
    apiRequest.mockResolvedValue({ ok: false, text: async () => '', json: async () => [] });
    getApplicationsPaginated.mockReset();
  });

  it('silent-обновление (SSE) не показывает оверлей refreshing', async () => {
    const resolvers = [];
    getApplicationsPaginated.mockImplementation(() => new Promise((r) => resolvers.push(r)));
    useAuthStore().token = 'test-token';
    const w = mountCenter();
    await nextTick();
    // чистый старт после mounted-вызовов
    w.vm.refreshing = false;
    w.vm.pendingRefreshCount = 0;
    resolvers.length = 0;

    w.vm.fetchApplications(true); // silent
    await flushPromises();
    expect(w.vm.refreshing).toBe(false);
  });

  it('не залипает при гонке silent(SSE) + non-silent(фильтр)', async () => {
    const resolvers = [];
    getApplicationsPaginated.mockImplementation(() => new Promise((r) => resolvers.push(r)));
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
    resolvers[1](page());
    await flushPromises();
    // non-silent резолвится позже: его seq устарел (N != N+1), но refreshing развязан
    // от seq (RED-фикс) - счётчик non-silent доходит до 0 и гасит оверлей.
    resolvers[0](page());
    await flushPromises();

    expect(w.vm.refreshing).toBe(false); // не залип
    expect(w.vm.pendingRefreshCount).toBe(0);
  });
});
