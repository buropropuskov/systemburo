import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { nextTick } from 'vue';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
// Список Центра (#1158) идёт через getApplicationsPaginated, не apiRequest напрямую.
vi.mock('@/api/applications', () => ({ getApplicationsPaginated: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));
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
  ApplicationsFilterModal: true,
  Badge: true,
  BaseDropdown: true,
};

function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn() } } },
  });
}

describe('ApplicationsCenter: мобильная шапка vs десктоп-инлайн (#1097 W3)', () => {
  let origMatchMedia;
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue({ ok: false, text: async () => '', json: async () => [] });
    getApplicationsPaginated.mockReset();
    getApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
    useAuthStore().token = 'test-token';
    origMatchMedia = window.matchMedia;
  });
  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('мобилка (matchMedia matches): модалка «Фильтр» и row2, без инлайн-фильтров', async () => {
    mockMatchMedia(true);
    const w = mountCenter();
    await nextTick();
    expect(w.vm.isMobileHeader).toBe(true);
    expect(w.find('.center__filters').exists()).toBe(false); // инлайн-фильтры скрыты
    expect(w.find('.header-row2').exists()).toBe(true); // мобильный второй ряд
    expect(w.find('[data-testid="center-button-filter"]').exists()).toBe(true);
    expect(w.find('.search-icon-btn').exists()).toBe(true); // поиск иконкой
  });

  it('десктоп (matchMedia не matches): инлайн-фильтры, без мобильной кнопки/ряда', async () => {
    mockMatchMedia(false);
    const w = mountCenter();
    await nextTick();
    expect(w.vm.isMobileHeader).toBe(false);
    expect(w.find('.center__filters').exists()).toBe(true); // инлайн-фильтры Центра
    expect(w.find('.header-row2').exists()).toBe(false);
    expect(w.find('.search-icon-btn').exists()).toBe(false);
    expect(w.find('[data-testid="center-button-filter"]').exists()).toBe(false);
  });

  it('toggleMobileSearch раскрывает/сворачивает поле поиска', async () => {
    mockMatchMedia(true);
    const w = mountCenter();
    await nextTick();
    expect(w.vm.showMobileSearch).toBe(false);
    expect(w.find('[data-testid="center-input-search"]').exists()).toBe(false);
    w.vm.toggleMobileSearch();
    await nextTick();
    expect(w.vm.showMobileSearch).toBe(true);
    expect(w.find('[data-testid="center-input-search"]').exists()).toBe(true);
  });

  it('индикатор «Фильтр» (hasModalFilters) загорается на выбранной организации', async () => {
    mockMatchMedia(true);
    const w = mountCenter();
    await nextTick();
    expect(w.vm.hasModalFilters).toBe(false);
    w.vm.selectedOrganizationId = 7;
    expect(w.vm.hasModalFilters).toBe(true);
  });

  it('messagePreview снимает HTML-теги rich-сообщения до плоского текста', async () => {
    mockMatchMedia(true);
    const w = mountCenter();
    await nextTick();
    expect(w.vm.messagePreview('<h1 class="heading-h1"><strong>Проведение работ</strong></h1>')).toBe(
      'Проведение работ',
    );
    expect(w.vm.messagePreview('<p>a</p>\n<p>b  c</p>')).toBe('a b c');
  });
});
