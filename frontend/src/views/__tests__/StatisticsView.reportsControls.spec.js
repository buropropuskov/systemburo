import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { reactive } from 'vue';

// У отчётов свой адрес (#2297): вкладка читается из маршрута и пишется в него,
// поэтому вью нужен роутер. Заглушка держит путь и переписывает его на push.
const { routeStub, pushSpy } = vi.hoisted(() => {
  const route = { path: '/analytics', params: {}, query: {} };
  return { routeStub: route, pushSpy: null };
});
const route = reactive(routeStub);
const push = vi.fn((to) => {
  if (typeof to === 'string') { route.path = to; route.query = {}; return; }
  route.path = to.path ?? route.path;
  route.query = to.query ?? {};
});
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push }) }));

// Вкладки монтируют тяжёлые панели с запросами — здесь они не нужны, проверяем шапку.
vi.mock('@/components/statistics/StatisticsDashboard.vue', () => ({ default: { name: 'StatisticsDashboard', template: '<div />' } }));
vi.mock('@/components/statistics/ProcessingAnalytics.vue', () => ({ default: { name: 'ProcessingAnalytics', template: '<div />' } }));
vi.mock('@/components/statistics/ReportsTab.vue', () => ({ default: { name: 'ReportsTab', template: '<div class="reports-stub" />' } }));

import StatisticsView from '../StatisticsView.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '@/components/RefreshButton.vue';

beforeEach(() => {
  route.path = '/analytics';
  route.query = {};
  push.mockClear();
});

function mountView() {
  return mount(StatisticsView, {
    global: {
      stubs: {
        AdminPageShell: { template: '<div><slot /></div>' },
        AnalyticsInstructionModal: true,
      },
    },
  });
}

async function openReports(wrapper) {
  const tab = wrapper.findAll('.statistics__tab').find((b) => b.text() === 'Отчёты');
  await tab.trigger('click');
  await flushPromises();
}

/*
 * Отчёт ведёт период в шаге 4 конструктора и строится по кнопке: шапочные пресеты,
 * календарь и «Обновить» на этой вкладке кликались и не делали ничего (#2295).
 */
describe('StatisticsView — шапка на вкладке «Отчёты»', () => {
  it('на дашборде период и обновление показаны', () => {
    const wrapper = mountView();
    expect(wrapper.find('.period-presets').isVisible()).toBe(true);
    expect(wrapper.findComponent(DateFilter).isVisible()).toBe(true);
    expect(wrapper.findComponent(RefreshButton).isVisible()).toBe(true);
  });

  it('на отчётах контролы, которые ни на что не влияли, скрыты', async () => {
    const wrapper = mountView();
    await openReports(wrapper);

    expect(wrapper.find('.reports-stub').exists()).toBe(true);
    expect(wrapper.find('.period-presets').isVisible()).toBe(false);
    expect(wrapper.findComponent(DateFilter).isVisible()).toBe(false);
    expect(wrapper.findComponent(RefreshButton).isVisible()).toBe(false);
  });

  it('переключение на отчёты меняет адрес, а прямая ссылка открывает вкладку сразу', async () => {
    const wrapper = mountView();
    await openReports(wrapper);
    expect(push).toHaveBeenCalledWith({ path: '/analytics', query: { tab: 'reports' } });

    route.query = { tab: 'reports' };
    const direct = mountView();
    await flushPromises();
    expect(direct.find('.reports-stub').exists()).toBe(true);
  });

  it('при возврате на дашборд контролы появляются снова', async () => {
    const wrapper = mountView();
    await openReports(wrapper);

    const dashboardTab = wrapper.findAll('.statistics__tab').find((b) => b.text() === 'Дашборд');
    await dashboardTab.trigger('click');
    await flushPromises();

    expect(wrapper.find('.period-presets').isVisible()).toBe(true);
    expect(wrapper.findComponent(DateFilter).isVisible()).toBe(true);
  });
});
