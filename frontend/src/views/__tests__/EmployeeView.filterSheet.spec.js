import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeView from '../EmployeeView.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import { usePermissionsStore } from '@/stores/permissions';

// S3 эпика mobile-filter-collapse (близнец CarsView): на мобилке (<=768) табы области
// сворачиваются в кнопку «Фильтр» + FilterSheet; поиск снаружи, десктоп-табы не тронуты.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listPersonBlacklist: vi.fn().mockResolvedValue([]) }));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  EmployeeEditModal: true,
  EmployeeDetailsModal: true,
  ApplicationDetail: true,
};

const origMatchMedia = window.matchMedia;
function setNarrow(matches) {
  window.matchMedia = () => ({
    matches,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
  });
}

function mountView() {
  const perms = usePermissionsStore();
  perms.mode = 'super';
  return mount(EmployeeView, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

async function ready(w, filter = 'user') {
  w.vm.loading = false;
  w.vm.currentFilter = filter;
  w.vm.ownershipInfo = {
    has_organization: true, has_company: true,
    user_id: 1, organization_id: 10, company_id: 20,
  };
  w.vm.fetchEmployees = vi.fn();
  await w.vm.$nextTick();
}

const filterBtn = w => w.find('[data-testid="employees-filter-btn"]');
const desktopOrgTab = w => w.find('[data-testid="filter-tab-organization"]');
const sheetOrgTab = w => w.find('[data-testid="employees-scope-organization"]');

let wrapper;

describe('EmployeeView — мобильный FilterSheet области (S3)', () => {
  beforeEach(() => setActivePinia(createPinia()));
  afterEach(() => {
    wrapper?.unmount();
    window.matchMedia = origMatchMedia;
  });

  describe('Гейт по ширине экрана', () => {
    it('мобилка: кнопка «Фильтр» есть, десктоп-табы скрыты', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper);
      expect(wrapper.vm.isNarrow).toBe(true);
      expect(filterBtn(wrapper).exists()).toBe(true);
      expect(desktopOrgTab(wrapper).exists()).toBe(false);
    });

    it('десктоп: десктоп-табы есть, кнопки «Фильтр» нет', async () => {
      setNarrow(false);
      wrapper = mountView();
      await ready(wrapper);
      expect(wrapper.vm.isNarrow).toBe(false);
      expect(desktopOrgTab(wrapper).exists()).toBe(true);
      expect(filterBtn(wrapper).exists()).toBe(false);
    });
  });

  describe('Точка-индикатор активной области', () => {
    it('scopeFilterActive=false на дефолтной области «Мои»', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'user');
      expect(wrapper.vm.scopeFilterActive).toBe(false);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(false);
    });

    it('scopeFilterActive=true на не-дефолтной области', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'organization');
      expect(wrapper.vm.scopeFilterActive).toBe(true);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(true);
    });
  });

  describe('Sheet: выбор области и сброс', () => {
    it('таб в sheet рендерится и его клик применяет область + закрывает лист', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'user');
      wrapper.vm.showScopeSheet = true;
      await wrapper.vm.$nextTick();
      const tab = sheetOrgTab(wrapper);
      expect(tab.exists()).toBe(true);
      await tab.trigger('click');
      expect(wrapper.vm.currentFilter).toBe('organization');
      expect(wrapper.vm.showScopeSheet).toBe(false);
      expect(wrapper.vm.fetchEmployees).toHaveBeenCalled();
    });

    it('resetScopeFilter возвращает область «Мои» и закрывает лист', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'all_system');
      wrapper.vm.showScopeSheet = true;
      wrapper.vm.resetScopeFilter();
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.currentFilter).toBe('user');
      expect(wrapper.vm.showScopeSheet).toBe(false);
    });

    it('поиск остаётся снаружи sheet (searchQuery не в scopeFilterActive)', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'user');
      wrapper.vm.searchQuery = 'ааа';
      await wrapper.vm.$nextTick();
      expect(wrapper.vm.hasActiveFilters).toBe(true);
      expect(wrapper.vm.scopeFilterActive).toBe(false);
    });
  });
});
