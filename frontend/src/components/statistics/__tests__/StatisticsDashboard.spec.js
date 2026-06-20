import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Управляемое состояние моков: фабрика vi.mock поднимается над импортами,
// поэтому summary/deferred выносим в hoisted.
const { state } = vi.hoisted(() => ({ state: { deferred: [], summary: {}, insights: {} } }));

vi.mock('@/api/statistics.js', () => ({
  getSummary: () => Promise.resolve(state.summary),
  getRecentPassages: () => Promise.resolve({ people: [], cars: [] }),
  getTimeline: () => new Promise((resolve) => { state.deferred.push(resolve); }),
  getInsights: () => Promise.resolve(state.insights),
}));

import StatisticsDashboard from '../StatisticsDashboard.vue';
import AnalyticsAreaChart from '../AnalyticsAreaChart.vue';
import Sparkline from '../Sparkline.vue';

const mountDashboard = () => mount(StatisticsDashboard, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: { stubs: { AnalyticsAreaChart: true, RefreshButton: true } },
});

beforeEach(() => {
  state.deferred.length = 0;
  state.summary = {};
  state.insights = {};
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
    expect(wrapper.findComponent(Sparkline).exists()).toBe(true);
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
