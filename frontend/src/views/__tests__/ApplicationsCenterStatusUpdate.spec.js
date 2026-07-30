import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { useAuthStore } from '@/stores/auth';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated } from '@/api/applications';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({
  getApplicationsPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
  getApplicationById: vi.fn(),
}));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));

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
  ApplicationsFilterModal: true,
};

function seedPerms() {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = {};
}

const busEmit = vi.fn();

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: {
      stubs,
      mocks: {
        $route: { query: {}, path: '/center' },
        $router: { push: vi.fn(), replace: vi.fn(() => Promise.resolve()) },
        $bus: { emit: busEmit, on: vi.fn(), off: vi.fn() },
      },
    },
  });
}

function fullApp(over = {}) {
  return {
    id: 1, is_read: true, application_number: 'A-1', organization_name: 'Орг',
    sender_name: 'И', sending_datetime: '2026-01-01T10:00:00Z',
    status: 'В работе', confirmation: 'Согласовано',
    ...over,
  };
}

let wrapper;

describe('ApplicationsCenter — флаг обновления статуса (#1349)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    seedPerms();
    busEmit.mockReset();
    apiRequest.mockReset();
    getApplicationsPaginated.mockClear();
    getApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });
  afterEach(() => wrapper?.unmount());

  it('прочитанная заявка с has_status_update: класс .status-updated + пульс-точка', async () => {
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    wrapper.vm.applications = [fullApp({ id: 3, is_read: true, has_status_update: true })];
    await wrapper.vm.$nextTick();

    const item = wrapper.find('[data-testid="center-row-3"]');
    expect(item.classes()).toContain('status-updated');
    expect(wrapper.find('[data-testid="center-status-dot-3"]').exists()).toBe(true);
  });

  it('НЕПРОЧИТАННАЯ заявка с has_status_update НЕ показывает точку (у неё своя жёлтая подсветка)', async () => {
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    wrapper.vm.applications = [fullApp({ id: 4, is_read: false, has_status_update: true })];
    await wrapper.vm.$nextTick();

    const item = wrapper.find('[data-testid="center-row-4"]');
    expect(item.classes()).toContain('unread');
    expect(item.classes()).not.toContain('status-updated');
    expect(wrapper.find('[data-testid="center-status-dot-4"]').exists()).toBe(false);
  });

  it('statusUpdateCount считает только прочитанные с флагом', () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [
      fullApp({ id: 1, is_read: true, has_status_update: true }),
      fullApp({ id: 2, is_read: true, has_status_update: false }),
      fullApp({ id: 3, is_read: false, has_status_update: true }), // непрочитана — не в счёте
    ];
    expect(wrapper.vm.statusUpdateCount).toBe(1);
  });

  it('чип "Обновления" шлёт серверный фильтр status_updated=true', async () => {
    wrapper = mountCenter();
    wrapper.vm.statusUpdatedOnly = true;
    await wrapper.vm.buildApplicationsPage(1, 30);
    const params = getApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBe('true');
  });

  it('без чипа параметр status_updated не отправляется', async () => {
    wrapper = mountCenter();
    wrapper.vm.statusUpdatedOnly = false;
    await wrapper.vm.buildApplicationsPage(1, 30);
    const params = getApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBeUndefined();
  });

  it('toggleStatusUpdated переключает флаг и рефетчит', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn'; // fetchApplications рано выходит без токена
    getApplicationsPaginated.mockClear();
    await wrapper.vm.toggleStatusUpdated();
    expect(wrapper.vm.statusUpdatedOnly).toBe(true);
    await wrapper.vm.$nextTick();
    expect(getApplicationsPaginated).toHaveBeenCalled();
  });

  it('resetFilters сбрасывает statusUpdatedOnly', () => {
    wrapper = mountCenter();
    wrapper.vm.statusUpdatedOnly = true;
    wrapper.vm.resetFilters();
    expect(wrapper.vm.statusUpdatedOnly).toBe(false);
  });

  it('openApplication оптимистично гасит флаг и эмитит application-read', async () => {
    wrapper = mountCenter();
    const app = fullApp({ id: 9, is_read: true, has_status_update: true });
    wrapper.vm.applications = [app];
    await wrapper.vm.openApplication(app);
    expect(app.has_status_update).toBe(false);
    expect(busEmit).toHaveBeenCalledWith('application-read', 9);
  });

  it('openApplication без флага не эмитит лишний application-read', async () => {
    wrapper = mountCenter();
    const app = fullApp({ id: 10, is_read: true, has_status_update: false });
    wrapper.vm.applications = [app];
    await wrapper.vm.openApplication(app);
    expect(busEmit).not.toHaveBeenCalledWith('application-read', 10);
  });

  it('открытие НЕпрочитанной заявки с флагом эмитит application-read РОВНО один раз', async () => {
    wrapper = mountCenter();
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({}) });
    const app = fullApp({ id: 11, is_read: false, has_status_update: true });
    wrapper.vm.applications = [app];
    await wrapper.vm.openApplication(app);
    expect(app.is_read).toBe(true);
    expect(app.has_status_update).toBe(false);
    // Прочтение + гашение флага сходятся в ОДИН эмит (не два запроса /unread-count).
    const reads = busEmit.mock.calls.filter(c => c[0] === 'application-read' && c[1] === 11);
    expect(reads.length).toBe(1);
  });

  it('инкрементальный синк подхватывает смену has_status_update', async () => {
    wrapper = mountCenter();
    const auth = useAuthStore();
    auth.token = 'tkn';
    wrapper.vm.applications = [fullApp({ id: 5, is_read: true, has_status_update: false })];
    // Свежий серверный снимок: у той же заявки флаг загорелся.
    apiRequest.mockResolvedValue({
      ok: true,
      json: async () => [fullApp({ id: 5, is_read: true, has_status_update: true })],
    });
    await wrapper.vm._pollApplicationsIncremental();
    expect(wrapper.vm.applications.find(a => a.id === 5).has_status_update).toBe(true);
  });
});
