import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import CarsView from '../CarsView.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import { usePermissionsStore } from '@/stores/permissions';

// S3 эпика mobile-filter-collapse: на мобилке (<=768, useNarrowScreen) табы области
// сворачиваются в кнопку «Фильтр» + FilterSheet; поиск остаётся снаружи, десктоп-табы
// не трогаются. isNarrow идёт от window.matchMedia - мокаем его до mount.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listVehicleBlacklist: vi.fn().mockResolvedValue([]) }));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  ConfirmationModal: true,
  VehicleDetailsModal: true,
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
  return mount(CarsView, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

/** Готовит владельческое состояние + гасит fetch, чтобы switchFilter не дёргал API. */
async function ready(w, filter = 'user') {
  w.vm.loading = false;
  w.vm.currentFilter = filter;
  w.vm.ownershipInfo = {
    has_organization: true, has_company: true,
    user_id: 1, organization_id: 10, company_id: 20,
  };
  w.vm.fetchCars = vi.fn();
  await w.vm.$nextTick();
}

const filterBtn = w => w.find('[data-testid="cars-filter-btn"]');
const desktopOrgTab = w => w.find('[data-testid="filter-tab-organization"]');
const sheetOrgTab = w => w.find('[data-testid="cars-scope-organization"]');

let wrapper;

describe('CarsView — мобильный FilterSheet области (S3)', () => {
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
    it('scopeFilterActive=false и точка погашена на дефолтной области «Мои»', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'user');
      expect(wrapper.vm.scopeFilterActive).toBe(false);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(false);
    });

    it('scopeFilterActive=true и точка горит на не-дефолтной области', async () => {
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
      expect(wrapper.vm.fetchCars).toHaveBeenCalled();
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

    it('поиск НЕ уходит в sheet: остаётся снаружи (searchQuery не в scopeFilterActive)', async () => {
      setNarrow(true);
      wrapper = mountView();
      await ready(wrapper, 'user');
      wrapper.vm.searchQuery = 'ааа';
      await wrapper.vm.$nextTick();
      // Поиск активен, но область дефолтная - индикатор области НЕ должен гореть.
      expect(wrapper.vm.hasActiveFilters).toBe(true);
      expect(wrapper.vm.scopeFilterActive).toBe(false);
    });
  });
});
