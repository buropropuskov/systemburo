import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Управляемое состояние моков: фабрика vi.mock поднимается над импортами,
// поэтому summary/deferred выносим в hoisted.
const { state } = vi.hoisted(() => ({
  state: { deferred: [], summary: {}, insights: {}, onlinePeaks: [] },
}));

vi.mock('@/api/statistics.js', () => ({
  getSummary: () => Promise.resolve(state.summary),
  getRecentPassages: () => Promise.resolve({ people: [], cars: [] }),
  getTimeline: () => new Promise((resolve) => { state.deferred.push(resolve); }),
  getInsights: () => Promise.resolve(state.insights),
  getOnlinePeaks: () => Promise.resolve(state.onlinePeaks),
}));

import StatisticsDashboard from '../StatisticsDashboard.vue';
import AnalyticsAreaChart from '../AnalyticsAreaChart.vue';
import AnalyticsBarChart from '../AnalyticsBarChart.vue';
import TrendSparkline from '../TrendSparkline.vue';

const mountDashboard = () => mount(StatisticsDashboard, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: { stubs: { AnalyticsAreaChart: true, AnalyticsBarChart: true, RefreshButton: true } },
});

const tileByText = (wrapper, label) =>
  wrapper.findAll('.dashboard__tile').find((t) => t.text().includes(label));

beforeEach(() => {
  state.deferred.length = 0;
  state.summary = {};
  state.insights = {};
  state.onlinePeaks = [];
});

describe('StatisticsDashboard — гонка таймлайна', () => {
  it('при быстрой смене метрики рисует данные последнего запроса, медленный предыдущий игнорирует', async () => {
    const wrapper = mountDashboard();
    await nextTick(); // onMounted -> loadTimeline (seq 1, deferred)

    // Быстрое переключение метрики -> второй loadTimeline (seq 2).
    await wrapper.findAll('.dashboard__seg-btn')[1].trigger('click');
    await nextTick();

    expect(state.deferred).toHaveLength(2);

    // Последний запрос (seq 2) приходит ПЕРВЫМ, устаревший seq 1 — позже.
    state.deferred[1]([{ date: '2026-06-02', count: 222 }]);
    await flushPromises();
    state.deferred[0]([{ date: '2026-06-01', count: 111 }]);
    await flushPromises();

    const chart = wrapper.findComponent(AnalyticsAreaChart);
    const data = chart.props('data');
    expect(data).toHaveLength(1);
    expect(data[0].count).toBe(222); // не 111 от устаревшего ответа
  });
});

describe('StatisticsDashboard — плитки', () => {
  it('показывает переименованные метрики и убирает снятые плитки', async () => {
    state.summary = { total_applications: 5, processed: 2, active_users: 7 };
    const wrapper = mountDashboard();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Получено заявок');
    expect(text).toContain('Пользователи');
    expect(text).not.toContain('Всего заявок');
    expect(text).not.toContain('В работе');
    expect(text).not.toContain('Сумма товаров');
    expect(text).not.toContain('Вложения (всего)');
  });

  it('раздел Вложения рендерит блок на каждый активный тип системы', async () => {
    state.summary = {
      by_attachment_type: [
        { name: 'Паспорт', count: 3 },
        { name: 'Виза', count: 0 },
      ],
    };
    const wrapper = mountDashboard();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Вложения');
    expect(text).toContain('Паспорт');
    expect(text).toContain('Виза');
  });
});

describe('StatisticsDashboard — инсайты карточек', () => {
  it('карточка с метрикой инсайта рендерит спарклайн и бейдж дельты; карточки без инсайта — нет', async () => {
    state.summary = { total_applications: 10, processed: 4, cars_entered: 8 };
    state.insights = {
      comparisons: [
        { metric: 'applications_count', current: 10, previous: 8, delta_pct: 25, direction: 'up' },
      ],
      trends: [
        { metric: 'applications_count', direction: 'up', series: [1, 2, 3, 4] },
      ],
    };
    const wrapper = mountDashboard();
    await flushPromises();

    // Только applications_count покрыт инсайтом -> ровно один футер с дельтой и спарклайном.
    expect(wrapper.text()).toContain('+25%');
    expect(wrapper.find('.dashboard__delta--up').exists()).toBe(true);
    expect(wrapper.findComponent(TrendSparkline).exists()).toBe(true);
    expect(wrapper.findAll('.dashboard__tile-insight')).toHaveLength(1);
  });

  it('без инсайтов карточки рендерятся без футера', async () => {
    state.summary = { total_applications: 10, cars_entered: 8 };
    state.insights = {};
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.findAll('.dashboard__tile-insight')).toHaveLength(0);
  });
});

describe('StatisticsDashboard — разворот карточки', () => {
  const withInsights = () => {
    state.summary = { total_applications: 10, cars_entered: 8 };
    state.insights = {
      comparisons: [{ metric: 'applications_count', current: 10, previous: 8, delta_pct: 25, direction: 'up' }],
      trends: [{ metric: 'applications_count', direction: 'up', series: [1, 2, 3, 4] }],
      peak_hours: [{
        metric: 'applications_count', label: 'Заявки', peak_hour: 9, peak_value: 5,
        hourly: [{ hour: 8, value: 2 }, { hour: 9, value: 5 }],
      }],
    };
  };

  it('обогащённая карточка кликабельна, клик раскрывает тренд и пик, повторный — сворачивает', async () => {
    withInsights();
    const wrapper = mountDashboard();
    await flushPromises();

    const tile = tileByText(wrapper, 'Получено заявок');
    expect(tile.classes()).toContain('dashboard__tile--clickable');
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);

    await tile.trigger('click');
    await nextTick();

    expect(wrapper.find('.dashboard__detail').exists()).toBe(true);
    expect(wrapper.find('.dashboard__detail').text()).toContain('Получено заявок');
    // Тренд (area) и пик по часам (bar) — оба графика в детальной панели.
    const detail = wrapper.find('.dashboard__detail');
    expect(detail.findComponent(AnalyticsAreaChart).exists()).toBe(true);
    expect(detail.findComponent(AnalyticsBarChart).exists()).toBe(true);
    expect(tile.classes()).toContain('dashboard__tile--active');

    await tile.trigger('click');
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);
  });

  it('тренд детали строит ту же серию, что и спарклайн, с порядковыми подписями оси X', async () => {
    withInsights();
    const wrapper = mountDashboard();
    await flushPromises();

    await tileByText(wrapper, 'Получено заявок').trigger('click');
    await nextTick();

    const area = wrapper.find('.dashboard__detail').findComponent(AnalyticsAreaChart);
    expect(area.props('data')).toEqual([{ count: 1 }, { count: 2 }, { count: 3 }, { count: 4 }]);
    expect(area.props('categories')).toEqual(['1', '2', '3', '4']);
  });

  it('смена периода сворачивает разворот', async () => {
    withInsights();
    const wrapper = mountDashboard();
    await flushPromises();

    await tileByText(wrapper, 'Получено заявок').trigger('click');
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(true);

    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);
  });

  it('карточка без инсайтов не кликабельна и не раскрывается', async () => {
    state.summary = { processed: 4 };
    state.insights = {};
    const wrapper = mountDashboard();
    await flushPromises();

    const tile = tileByText(wrapper, 'Обработано');
    expect(tile).toBeTruthy();
    expect(tile.classes()).not.toContain('dashboard__tile--clickable');
    await tile.trigger('click');
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);
  });
});

describe('StatisticsDashboard — динамика онлайна', () => {
  it('рендерит area-график пиков онлайна за период', async () => {
    state.onlinePeaks = [
      { date: '2026-06-01', peak: 4 },
      { date: '2026-06-02', peak: 9 },
    ];
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.text()).toContain('Динамика онлайна');
    const online = wrapper.findAllComponents(AnalyticsAreaChart)
      .find((c) => c.props('seriesName') === 'Пик онлайна');
    expect(online).toBeTruthy();
    expect(online.props('data')).toEqual([
      { timestamp: '2026-06-01', count: 4 },
      { timestamp: '2026-06-02', count: 9 },
    ]);
  });
});
