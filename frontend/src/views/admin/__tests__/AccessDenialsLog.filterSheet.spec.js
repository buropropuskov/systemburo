import {
  describe, it, expect, vi, beforeEach, afterEach,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import AccessDenialsLog from '../AccessDenialsLog.vue';
import FilterButton from '@/components/ui/FilterButton.vue';

// S4 эпика mobile-filter-collapse: на мобилке (<=768, useNarrowScreen) форма
// фильтров сворачивается в кнопку «Фильтр» + FilterSheet; переключатель
// Активные/Архив и bulk-действия остаются снаружи (это не фильтры). Десктоп не
// трогается. isNarrow идёт от window.matchMedia - мокаем его до mount.
const listAccessDenials = vi.fn();
const listAccessDenialsArchive = vi.fn();
vi.mock('@/api/permissions', () => ({
  listAccessDenials: (...a) => listAccessDenials(...a),
  listAccessDenialsArchive: (...a) => listAccessDenialsArchive(...a),
  deleteAccessDenials: vi.fn(),
  archiveAccessDenials: vi.fn(),
}));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: vi.fn() }) }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: vi.fn() }) }));

const stubs = {
  teleport: true, RefreshButton: true, BaseDropdown: true, Pager: true,
};

function pageData(items = [], over = {}) {
  return {
    items, total: items.length, page: 1, limit: 50, ...over,
  };
}

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

async function mountLog() {
  const wrapper = mount(AccessDenialsLog, { global: { stubs } });
  await flushPromises();
  return wrapper;
}

const filterBtn = w => w.find('[data-testid="denials-filter-btn"]');
const desktopForm = w => w.find('form.filters');

let wrapper;

describe('AccessDenialsLog — мобильный FilterSheet (S4)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listAccessDenials.mockResolvedValue(pageData());
    listAccessDenialsArchive.mockResolvedValue(pageData());
  });
  afterEach(() => {
    wrapper?.unmount();
    window.matchMedia = origMatchMedia;
  });

  describe('Гейт по ширине экрана', () => {
    it('мобилка: кнопка «Фильтр» есть, десктоп-форма скрыта', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      expect(wrapper.vm.isNarrow).toBe(true);
      expect(filterBtn(wrapper).exists()).toBe(true);
      expect(desktopForm(wrapper).exists()).toBe(false);
    });

    it('десктоп: форма есть, кнопки «Фильтр» нет', async () => {
      setNarrow(false);
      wrapper = await mountLog();
      expect(wrapper.vm.isNarrow).toBe(false);
      expect(desktopForm(wrapper).exists()).toBe(true);
      expect(filterBtn(wrapper).exists()).toBe(false);
    });

    it('переключатель Активные/Архив остаётся снаружи и на мобилке', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      expect(wrapper.find('.toggle-row').exists()).toBe(true);
    });
  });

  describe('Точка-индикатор применённых фильтров', () => {
    it('точка погашена без фильтров', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      expect(wrapper.vm.hasActiveFilters).toBe(false);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(false);
    });

    it('точка горит после применённого фильтра', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      await wrapper.setData({ filters: { ...wrapper.vm.filters, resource: '/api/employees' } });
      expect(wrapper.vm.hasActiveFilters).toBe(true);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(true);
    });
  });

  describe('Sheet: применение, сброс и откат черновика', () => {
    it('«Применить» шлёт фильтры, сбрасывает страницу и закрывает лист', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      wrapper.vm.openFilterSheet();
      await wrapper.vm.$nextTick();
      await wrapper.find('[data-testid="denials-sheet-resource"]').setValue('/api/cars');
      await wrapper.find('[data-testid="denials-sheet-apply"]').trigger('click');
      await flushPromises();

      expect(listAccessDenials).toHaveBeenLastCalledWith(expect.objectContaining({
        page: 1, resource: '/api/cars',
      }));
      expect(wrapper.vm.showFilterSheet).toBe(false);
    });

    it('закрытие без «Применить» откатывает неприменённый черновик', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      wrapper.vm.filters.resource = '/applied';
      wrapper.vm.applyFilters();
      await flushPromises();
      wrapper.vm.openFilterSheet();
      wrapper.vm.filters.resource = '/draft';
      wrapper.vm.closeFilterSheet();
      await wrapper.vm.$nextTick();

      expect(wrapper.vm.filters.resource).toBe('/applied');
      expect(wrapper.vm.showFilterSheet).toBe(false);
    });
  });
});
