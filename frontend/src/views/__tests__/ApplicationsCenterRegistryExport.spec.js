import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { downloadApplicationsRegistry } from '@/api/applications';

// Выгрузка реестра заявок (#1832). Проверяется гейт по праву и то, что в выгрузку
// уезжают ТЕКУЩИЕ фильтры экрана: иначе кнопка отдаёт не то, что человек отобрал.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }) }));
vi.mock('@/api/applications', () => ({
  getApplicationsPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
  getApplicationById: vi.fn(),
  downloadApplicationsRegistry: vi.fn().mockResolvedValue(undefined),
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
};

function seedPerms({ allow = [] } = {}) {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
}

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  });
}

describe('ApplicationsCenter - выгрузка реестра (#1832)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('без права «Экспорт заявок» кнопки выгрузки нет ни в одной раскладке', () => {
    seedPerms({ allow: ['page.center'] });
    const w = mountCenter();
    expect(w.find('[data-testid="center-button-export-desktop"]').exists()).toBe(false);
    expect(w.find('[data-testid="center-button-export"]').exists()).toBe(false);
  });

  it('с правом кнопка появляется на десктопе', () => {
    seedPerms({ allow: ['page.center', 'action.export.applications'] });
    const w = mountCenter();
    w.vm.isMobileHeader = false;
    expect(w.find('[data-testid="center-button-export-desktop"]').exists()).toBe(true);
  });

  it('в выгрузку уезжают текущие фильтры экрана, а не пустой запрос', async () => {
    seedPerms({ allow: ['page.center', 'action.export.applications'] });
    const w = mountCenter();
    w.vm.isMobileHeader = false;
    w.vm.searchQuery = 'Иванов';
    w.vm.selectedOrganizationIds = [7, 9];
    w.vm.selectedApplicationStatuses = ['В обработке'];
    await w.vm.$nextTick();

    await w.find('[data-testid="center-button-export-desktop"]').trigger('click');

    expect(downloadApplicationsRegistry).toHaveBeenCalledTimes(1);
    const params = downloadApplicationsRegistry.mock.calls[0][0];
    expect(params.search_query).toBe('Иванов');
    expect(params.organization_ids).toBe('7,9');
    expect(params.status).toBe('В обработке');
    // page/per_page - забота листинга: в выгрузку страница не передаётся, иначе
    // в файл уехала бы только первая порция.
    expect(params.page).toBeUndefined();
    expect(params.per_page).toBeUndefined();
  });

  it('псевдо-статус «Непрочитано» уезжает флагом unread, как в списке', async () => {
    seedPerms({ allow: ['page.center', 'action.export.applications'] });
    const w = mountCenter();
    w.vm.isMobileHeader = false;
    w.vm.selectedApplicationStatuses = ['Непрочитано'];
    await w.vm.$nextTick();

    await w.find('[data-testid="center-button-export-desktop"]').trigger('click');

    const params = downloadApplicationsRegistry.mock.calls[0][0];
    expect(params.unread).toBe('true');
    expect(params.status).toBeUndefined();
  });

  it('повторный клик во время сборки файла запрос не дублирует', async () => {
    seedPerms({ allow: ['page.center', 'action.export.applications'] });
    let release;
    downloadApplicationsRegistry.mockImplementationOnce(() => new Promise((res) => { release = res; }));
    const w = mountCenter();
    w.vm.isMobileHeader = false;
    await w.vm.$nextTick();

    const btn = w.find('[data-testid="center-button-export-desktop"]');
    await btn.trigger('click');
    await btn.trigger('click');
    expect(downloadApplicationsRegistry).toHaveBeenCalledTimes(1);

    release();
    await w.vm.$nextTick();
    expect(w.vm.exporting).toBe(false);
  });

  it('сбой выгрузки не оставляет кнопку заблокированной', async () => {
    seedPerms({ allow: ['page.center', 'action.export.applications'] });
    downloadApplicationsRegistry.mockRejectedValueOnce(new Error('500'));
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const w = mountCenter();
    w.vm.isMobileHeader = false;
    await w.vm.$nextTick();

    await w.find('[data-testid="center-button-export-desktop"]').trigger('click');
    await w.vm.$nextTick();

    expect(w.vm.exporting).toBe(false);
  });
});
