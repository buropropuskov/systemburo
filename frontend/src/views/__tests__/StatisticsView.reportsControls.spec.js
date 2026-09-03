import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// Вкладки монтируют тяжёлые панели с запросами — здесь они не нужны, проверяем шапку.
vi.mock('@/components/statistics/StatisticsDashboard.vue', () => ({ default: { name: 'StatisticsDashboard', template: '<div />' } }));
vi.mock('@/components/statistics/ProcessingAnalytics.vue', () => ({ default: { name: 'ProcessingAnalytics', template: '<div />' } }));
vi.mock('@/components/statistics/ReportsTab.vue', () => ({ default: { name: 'ReportsTab', template: '<div class="reports-stub" />' } }));

import StatisticsView from '../StatisticsView.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '@/components/RefreshButton.vue';

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
