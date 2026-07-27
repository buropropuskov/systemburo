import {
  describe, it, expect, vi, beforeEach, afterEach,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import PdAuditLog from '../PdAuditLog.vue';
import FilterButton from '@/components/ui/FilterButton.vue';

// S4 эпика mobile-filter-collapse: у журнала 152-ФЗ нет отдельного поиска - весь
// тулбар это форма фильтров, применяемая по «Применить». На мобилке (<=768,
// useNarrowScreen) форма сворачивается в кнопку «Фильтр» + FilterSheet; десктоп
// не трогается. isNarrow идёт от window.matchMedia - мокаем его до mount.
const listPDAudit = vi.fn();
vi.mock('@/api/pd-audit', () => ({ listPDAudit: (...a) => listPDAudit(...a) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: vi.fn() }) }));

const stubs = { teleport: true, RefreshButton: true, BaseDropdown: true };

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
  const wrapper = mount(PdAuditLog, { global: { stubs } });
  await flushPromises();
  return wrapper;
}

const filterBtn = w => w.find('[data-testid="pda-filter-btn"]');
const desktopApply = w => w.find('[data-testid="pda-apply"]');

let wrapper;

describe('PdAuditLog — мобильный FilterSheet (S4)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listPDAudit.mockResolvedValue(pageData());
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
      expect(desktopApply(wrapper).exists()).toBe(false);
    });

    it('десктоп: форма есть, кнопки «Фильтр» нет', async () => {
      setNarrow(false);
      wrapper = await mountLog();
      expect(wrapper.vm.isNarrow).toBe(false);
      expect(desktopApply(wrapper).exists()).toBe(true);
      expect(filterBtn(wrapper).exists()).toBe(false);
    });
  });

  describe('Точка-индикатор применённых фильтров', () => {
    it('точка погашена без фильтров', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      expect(wrapper.vm.filtersActive).toBe(false);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(false);
    });

    it('точка горит после применённого фильтра (в т.ч. только «Только отказы»)', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      await wrapper.setData({ filters: { ...wrapper.vm.filters, only_denied: true } });
      expect(wrapper.vm.filtersActive).toBe(true);
      expect(wrapper.findComponent(FilterButton).props('active')).toBe(true);
    });
  });

  describe('Sheet: применение, сброс и откат черновика', () => {
    it('«Применить» шлёт фильтры, сбрасывает страницу и закрывает лист', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      wrapper.vm.openFilterSheet();
      await wrapper.vm.$nextTick();
      await wrapper.find('[data-testid="pda-sheet-username"]').setValue('kafanova');
      await wrapper.find('[data-testid="pda-sheet-apply"]').trigger('click');
      await flushPromises();

      expect(listPDAudit).toHaveBeenLastCalledWith(expect.objectContaining({
        page: 1, username: 'kafanova',
      }));
      expect(wrapper.vm.showFilterSheet).toBe(false);
    });

    it('«Сбросить» очищает фильтры и закрывает лист', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      await wrapper.setData({ filters: { ...wrapper.vm.filters, username: 'kafanova' } });
      wrapper.vm.openFilterSheet();
      await wrapper.vm.$nextTick();
      await wrapper.find('[data-testid="pda-sheet-reset"]').trigger('click');
      await flushPromises();

      expect(wrapper.vm.filters.username).toBe('');
      expect(wrapper.vm.showFilterSheet).toBe(false);
    });

    it('закрытие без «Применить» откатывает неприменённый черновик', async () => {
      setNarrow(true);
      wrapper = await mountLog();
      // применили один фильтр
      wrapper.vm.filters.username = 'applied';
      wrapper.vm.applyFilters();
      await flushPromises();
      // открыли лист, поменяли черновик, закрыли крестиком/overlay/Escape
      wrapper.vm.openFilterSheet();
      wrapper.vm.filters.username = 'draft';
      wrapper.vm.closeFilterSheet();
      await wrapper.vm.$nextTick();

      expect(wrapper.vm.filters.username).toBe('applied');
      expect(wrapper.vm.showFilterSheet).toBe(false);
    });
  });
});
