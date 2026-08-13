import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeView from '../EmployeeView.vue';
import CarsView from '../CarsView.vue';
import { usePermissionsStore } from '@/stores/permissions';

/**
 * Волна 5 мобильной раскладки: «Добавить» уходит из шапки списка в панель у нижнего
 * края экрана, а поиск с «Фильтром» переезжают отдельной полосой под шапку.
 *
 * Переезд сделан через v-if (элемент физически один), поэтому проверяем не «кнопка
 * где-то есть», а ГДЕ именно она лежит и что в DOM она одна: скрытая копия задвоила бы
 * data-testid и сломала якорь онбординг-тура. Отдельно закрываем случай, ради которого
 * этот замок и написан (#1915): условие контейнера, унаследованное элементом, способно
 * убрать действие с экрана целиком - на мобилке проверять кнопку негде, кроме панели.
 *
 * Геометрию (48px шапка, 36px полоса, бейджи 28px) юнит не видит - jsdom не считает
 * ни каскад, ни медиа-запросы; она проверяется замером в браузере.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({
  listPersonBlacklist: vi.fn().mockResolvedValue([]),
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
}));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  ConfirmationModal: true,
  VehicleDetailsModal: true,
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

const SCREENS = [
  {
    name: 'Сотрудники',
    component: EmployeeView,
    addBtn: 'ob-employees-add-button',
    toolbar: 'ob-employees-filters',
    count: 'employees-count-badge',
    card: 'ob-employees-table',
    head: 'ob-employees-table-head',
    bar: '.employeesview__action-bar',
    totalField: 'employeesTotal',
    label: 'Добавить сотрудника',
    quiet: w => { w.vm.fetchEmployees = vi.fn(); },
  },
  {
    name: 'Автомобили',
    component: CarsView,
    addBtn: 'cars-view-add-button',
    toolbar: 'ob-cars-filters',
    count: 'cars-count-badge',
    card: 'ob-cars-table',
    head: 'ob-cars-table-head',
    bar: '.carsview__action-bar',
    totalField: 'carsTotal',
    label: 'Добавить машину',
    quiet: w => { w.vm.fetchCars = vi.fn(); },
  },
];

describe.each(SCREENS)('$name - мобильная панель действий', (screen) => {
  let wrapper;

  async function mountReady({ narrow, filter = 'user', mode = 'super' }) {
    setNarrow(narrow);
    usePermissionsStore().mode = mode;
    const w = mount(screen.component, {
      global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
    });
    w.vm.loading = false;
    w.vm.currentFilter = filter;
    w.vm.ownershipInfo = {
      has_organization: true, has_company: true,
      user_id: 1, organization_id: 10, company_id: 20,
    };
    screen.quiet(w);
    await w.vm.$nextTick();
    return w;
  }

  beforeEach(() => setActivePinia(createPinia()));
  afterEach(() => {
    wrapper?.unmount();
    window.matchMedia = origMatchMedia;
  });

  it('мобилка: «Добавить» одно, лежит в нижней панели и подписано полностью', async () => {
    wrapper = await mountReady({ narrow: true });

    const found = wrapper.findAll(`[data-testid="${screen.addBtn}"]`);
    expect(found).toHaveLength(1);
    expect(found[0].text()).toBe(screen.label);

    const bar = wrapper.find(screen.bar);
    expect(bar.exists()).toBe(true);
    expect(bar.find(`[data-testid="${screen.addBtn}"]`).exists()).toBe(true);
    // Контракт для ScrollTopButton: по этому атрибуту кнопка «наверх» уходит выше панели.
    expect(bar.attributes('data-bottom-action-bar')).toBeDefined();

    // В шапке списка действий, кроме «Обновить», не осталось.
    expect(
      wrapper.find(`[data-testid="${screen.head}"]`).find(`[data-testid="${screen.addBtn}"]`).exists(),
    ).toBe(false);
  });

  it('мобилка: поиск с «Фильтром» - полоса внутри карточки списка, под её шапкой', async () => {
    wrapper = await mountReady({ narrow: true });

    const toolbars = wrapper.findAll(`[data-testid="${screen.toolbar}"]`);
    expect(toolbars).toHaveLength(1);
    expect(
      wrapper.find(`[data-testid="${screen.card}"]`).find(`[data-testid="${screen.toolbar}"]`).exists(),
    ).toBe(true);

    const html = wrapper.find(`[data-testid="${screen.card}"]`).html();
    expect(html.indexOf(screen.head)).toBeLessThan(html.indexOf(screen.toolbar));
  });

  it('мобилка: счётчик рядом с заголовком показывает серверный total', async () => {
    wrapper = await mountReady({ narrow: true });
    wrapper.vm[screen.totalField] = 18;
    await wrapper.vm.$nextTick();

    expect(wrapper.find(`[data-testid="${screen.count}"]`).text()).toBe('18');
  });

  it('десктоп: нижней панели нет, «Добавить» и полный блок фильтров на прежних местах', async () => {
    wrapper = await mountReady({ narrow: false });

    expect(wrapper.find(screen.bar).exists()).toBe(false);
    expect(wrapper.find(`[data-testid="${screen.count}"]`).exists()).toBe(false);

    const found = wrapper.findAll(`[data-testid="${screen.addBtn}"]`);
    expect(found).toHaveLength(1);
    expect(found[0].text()).toBe('Добавить');
    expect(
      wrapper.find(`[data-testid="${screen.head}"]`).find(`[data-testid="${screen.addBtn}"]`).exists(),
    ).toBe(true);

    // Фильтры на десктопе остаются НАД карточкой, а не внутри неё.
    expect(
      wrapper.find(`[data-testid="${screen.card}"]`).find(`[data-testid="${screen.toolbar}"]`).exists(),
    ).toBe(false);
    expect(wrapper.find(`[data-testid="${screen.toolbar}"]`).exists()).toBe(true);
  });

  it('мобилка: во «Всех записях системы» панели нет - там нечего добавлять', async () => {
    wrapper = await mountReady({ narrow: true, filter: 'all_system' });

    expect(wrapper.find(screen.bar).exists()).toBe(false);
    expect(wrapper.find(`[data-testid="${screen.addBtn}"]`).exists()).toBe(false);
    // Поиск при этом остаётся: он не зависит от права на запись.
    expect(wrapper.find(`[data-testid="${screen.toolbar}"]`).exists()).toBe(true);
  });
});
